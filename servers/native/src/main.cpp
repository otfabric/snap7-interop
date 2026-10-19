#include "fixture.hpp"
#include "log.hpp"
#include "state.hpp"

#include <atomic>
#include <chrono>
#include <cstdint>
#include <csignal>
#include <cstdlib>
#include <cstring>
#include <iostream>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

#include "snap7.h"

namespace {

std::atomic<bool> g_stop{false};

void OnSignal(int) { g_stop.store(true); }

std::string EnvOr(const char* key, const char* fallback) {
  const char* v = std::getenv(key);
  return (v && *v) ? std::string(v) : std::string(fallback);
}

int AreaCodeFor(const std::string& type) {
  if (type == "db") {
    return srvAreaDB;
  }
  if (type == "markers") {
    return srvAreaMK;
  }
  if (type == "inputs") {
    return srvAreaPE;
  }
  if (type == "outputs") {
    return srvAreaPA;
  }
  return -1;
}

// S7 area codes used in RW callbacks / PDU addressing.
byte S7AreaCodeFor(const std::string& type) {
  if (type == "inputs") {
    return 0x81;
  }
  if (type == "outputs") {
    return 0x82;
  }
  if (type == "markers") {
    return 0x83;
  }
  if (type == "db") {
    return 0x84;
  }
  return 0;
}

struct RWContext {
  std::vector<snap7_interop::AreaBuffer>* areas = nullptr;
  std::mutex mu;
};

bool FixtureHasReadOnly(const snap7_interop::CompiledFixture& fixture) {
  for (const auto& a : fixture.areas) {
    if (a.access == "read-only") {
      return true;
    }
  }
  return false;
}

snap7_interop::AreaBuffer* FindArea(RWContext* ctx, int area, int db_number) {
  if (ctx == nullptr || ctx->areas == nullptr) {
    return nullptr;
  }
  for (auto& a : *ctx->areas) {
    const byte code = S7AreaCodeFor(a.type);
    if (code != static_cast<byte>(area)) {
      continue;
    }
    if (a.type == "db") {
      if (a.number == db_number) {
        return &a;
      }
    } else {
      return &a;
    }
  }
  return nullptr;
}

int DataSizeByte(int word_len) {
  switch (word_len) {
    case S7WLBit:
      return 1;
    case S7WLByte:
      return 1;
    case S7WLWord:
    case S7WLCounter:
    case S7WLTimer:
      return 2;
    case S7WLDWord:
    case S7WLReal:
      return 4;
    default:
      return 0;
  }
}

int S7API RWAreaCallBack(void* usrPtr, int /*Sender*/, int Operation, PS7Tag PTag,
                         void* pUsrData) {
  auto* ctx = static_cast<RWContext*>(usrPtr);
  if (ctx == nullptr || PTag == nullptr || pUsrData == nullptr) {
    return -1;
  }

  std::lock_guard<std::mutex> lock(ctx->mu);
  auto* area = FindArea(ctx, PTag->Area, PTag->DBNumber);
  if (area == nullptr) {
    return -1;
  }

  const int elem_size = DataSizeByte(PTag->WordLen);
  if (elem_size <= 0 || PTag->Size <= 0) {
    return -1;
  }

  if (PTag->WordLen == S7WLBit) {
    // Start is bit address; Size is element count (must be 1).
    if (PTag->Size != 1) {
      return -1;
    }
    const int byte_off = PTag->Start >> 3;
    const int bit_idx = PTag->Start & 0x07;
    if (byte_off < 0 || byte_off >= static_cast<int>(area->data.size())) {
      return -1;
    }
    auto* buf = static_cast<pbyte>(pUsrData);
    if (Operation == OperationRead) {
      buf[0] = (area->data[static_cast<size_t>(byte_off)] >> bit_idx) & 0x01;
      return 0;
    }
    if (area->access == "read-only") {
      return -1;
    }
    if ((buf[0] & 0x01) != 0) {
      area->data[static_cast<size_t>(byte_off)] |=
          static_cast<uint8_t>(1u << bit_idx);
    } else {
      area->data[static_cast<size_t>(byte_off)] &=
          static_cast<uint8_t>(~(1u << bit_idx));
    }
    return 0;
  }

  const int start = PTag->Start;  // byte offset for non-bit
  const int nbytes = elem_size * PTag->Size;
  if (start < 0 || nbytes <= 0 ||
      start + nbytes > static_cast<int>(area->data.size())) {
    return -1;
  }

  auto* buf = static_cast<pbyte>(pUsrData);
  if (Operation == OperationRead) {
    std::memcpy(buf, area->data.data() + start, static_cast<size_t>(nbytes));
    return 0;
  }
  if (area->access == "read-only") {
    return -1;
  }
  std::memcpy(area->data.data() + start, buf, static_cast<size_t>(nbytes));
  return 0;
}

}  // namespace

int main(int argc, char** argv) {
  (void)argc;
  (void)argv;

  const std::string fixture_path = EnvOr("SNAP_INTEROP_FIXTURE", "");
  const std::string listen_addr = EnvOr("SNAP_INTEROP_LISTEN_ADDRESS", "0.0.0.0");
  const std::string port_s = EnvOr("SNAP_INTEROP_PORT", "102");
  const std::string state_path =
      EnvOr("SNAP_INTEROP_STATE_PATH", "/run/snap-interop/state.json");

  if (fixture_path.empty()) {
    std::cerr << "SNAP_INTEROP_FIXTURE is required\n";
    return 2;
  }

  int port = 0;
  try {
    port = std::stoi(port_s);
  } catch (...) {
    std::cerr << "invalid SNAP_INTEROP_PORT\n";
    return 2;
  }
  if (port <= 0 || port > 65535) {
    std::cerr << "SNAP_INTEROP_PORT out of range\n";
    return 2;
  }

  snap7_interop::CompiledFixture fixture;
  try {
    fixture = snap7_interop::LoadCompiledFixture(fixture_path);
  } catch (const std::exception& ex) {
    snap7_interop::LogJSON("error", "fatal_error", "unknown",
                          std::string("\"error\": \"") + ex.what() + "\"");
    return 1;
  }

  snap7_interop::LogJSON(
      "info", "fixture_loaded", fixture.id,
      "\"compiled_sha256\": \"" + fixture.compiled_sha256 + "\"");

  snap7_interop::RuntimeCounters counters;
  try {
    snap7_interop::WriteStateSnapshot(state_path, fixture, "starting", counters);
  } catch (const std::exception& ex) {
    snap7_interop::LogJSON("error", "fatal_error", fixture.id,
                          std::string("\"error\": \"") + ex.what() + "\"");
    return 1;
  }

  S7Object server = Srv_Create();
  if (server == 0) {
    snap7_interop::LogJSON("error", "fatal_error", fixture.id,
                          "\"error\": \"Srv_Create failed\"");
    return 1;
  }

  // Configure port before start.
  uint16_t local_port = static_cast<uint16_t>(port);
  int err = Srv_SetParam(server, p_u16_LocalPort, &local_port);
  if (err != 0) {
    char text[256];
    Srv_ErrorText(err, text, sizeof(text));
    snap7_interop::LogJSON("error", "fatal_error", fixture.id,
                          std::string("\"error\": \"Srv_SetParam port: ") +
                              text + "\"");
    Srv_Destroy(&server);
    return 1;
  }

  if (fixture.pdu_size > 0) {
    int32_t pdu = fixture.pdu_size;
    err = Srv_SetParam(server, p_i32_PDURequest, &pdu);
    if (err != 0) {
      snap7_interop::LogJSON(
          "warn", "fixture_loaded", fixture.id,
          "\"warning\": \"PDU size param not applied\", \"code\": " +
              std::to_string(err));
    }
  }

  RWContext rw_ctx;
  rw_ctx.areas = &fixture.areas;
  const bool resource_less = FixtureHasReadOnly(fixture);

  if (resource_less) {
    err = Srv_SetRWAreaCallback(server, RWAreaCallBack, &rw_ctx);
    if (err != 0) {
      char text[256];
      Srv_ErrorText(err, text, sizeof(text));
      snap7_interop::LogJSON(
          "error", "fatal_error", fixture.id,
          std::string("\"error\": \"Srv_SetRWAreaCallback: ") + text + "\"");
      Srv_Destroy(&server);
      return 1;
    }
    snap7_interop::LogJSON("info", "fixture_loaded", fixture.id,
                          "\"mode\": \"resource-less-readonly\"");
  } else {
    for (auto& area : fixture.areas) {
      const int code = AreaCodeFor(area.type);
      if (code < 0) {
        snap7_interop::LogJSON("error", "fatal_error", fixture.id,
                              "\"error\": \"unsupported area type " + area.type +
                                  "\"");
        Srv_Destroy(&server);
        return 1;
      }
      word index = 0;
      if (area.type == "db") {
        if (area.number < 1 || area.number > 65535) {
          snap7_interop::LogJSON("error", "fatal_error", fixture.id,
                                "\"error\": \"invalid DB number\"");
          Srv_Destroy(&server);
          return 1;
        }
        index = static_cast<word>(area.number);
      }
      // Snap7 keeps this pointer for the server lifetime — do not reallocate.
      err = Srv_RegisterArea(server, code, index, area.data.data(),
                             static_cast<int>(area.data.size()));
      if (err != 0) {
        char text[256];
        Srv_ErrorText(err, text, sizeof(text));
        snap7_interop::LogJSON("error", "fatal_error", fixture.id,
                              std::string("\"error\": \"Srv_RegisterArea: ") +
                                  text + "\"");
        Srv_Destroy(&server);
        return 1;
      }
    }
  }

  std::signal(SIGTERM, OnSignal);
  std::signal(SIGINT, OnSignal);

  snap7_interop::LogJSON(
      "info", "server_starting", fixture.id,
      "\"listen_address\": \"" + listen_addr +
          "\", \"port\": " + std::to_string(port));

  err = Srv_StartTo(server, listen_addr.c_str());
  if (err != 0) {
    char text[256];
    Srv_ErrorText(err, text, sizeof(text));
    snap7_interop::LogJSON("error", "fatal_error", fixture.id,
                          std::string("\"error\": \"Srv_StartTo: ") + text +
                              "\"");
    Srv_Destroy(&server);
    return 1;
  }

  try {
    snap7_interop::WriteStateSnapshot(state_path, fixture, "ready", counters);
  } catch (const std::exception& ex) {
    snap7_interop::LogJSON("error", "fatal_error", fixture.id,
                          std::string("\"error\": \"") + ex.what() + "\"");
    Srv_Stop(server);
    Srv_Destroy(&server);
    return 1;
  }

  snap7_interop::LogJSON("info", "server_ready", fixture.id,
                        "\"port\": " + std::to_string(port));

  while (!g_stop.load()) {
    // Periodically refresh diagnostic snapshot so writes are visible.
    try {
      snap7_interop::WriteStateSnapshot(state_path, fixture, "ready", counters);
    } catch (...) {
      // Best-effort diagnostics must not kill the server.
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(500));
  }

  snap7_interop::LogJSON("info", "server_stopping", fixture.id, "{}");
  Srv_Stop(server);
  try {
    snap7_interop::WriteStateSnapshot(state_path, fixture, "stopped", counters);
  } catch (...) {
  }
  Srv_Destroy(&server);
  snap7_interop::LogJSON("info", "server_stopped", fixture.id, "{}");
  return 0;
}
