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
#include <string>
#include <thread>

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

  snap_interop::CompiledFixture fixture;
  try {
    fixture = snap_interop::LoadCompiledFixture(fixture_path);
  } catch (const std::exception& ex) {
    snap_interop::LogJSON("error", "fatal_error", "unknown",
                          std::string("\"error\": \"") + ex.what() + "\"");
    return 1;
  }

  snap_interop::LogJSON(
      "info", "fixture_loaded", fixture.id,
      "\"compiled_sha256\": \"" + fixture.compiled_sha256 + "\"");

  snap_interop::RuntimeCounters counters;
  try {
    snap_interop::WriteStateSnapshot(state_path, fixture, "starting", counters);
  } catch (const std::exception& ex) {
    snap_interop::LogJSON("error", "fatal_error", fixture.id,
                          std::string("\"error\": \"") + ex.what() + "\"");
    return 1;
  }

  S7Object server = Srv_Create();
  if (server == 0) {
    snap_interop::LogJSON("error", "fatal_error", fixture.id,
                          "\"error\": \"Srv_Create failed\"");
    return 1;
  }

  // Configure port before start.
  uint16_t local_port = static_cast<uint16_t>(port);
  int err = Srv_SetParam(server, p_u16_LocalPort, &local_port);
  if (err != 0) {
    char text[256];
    Srv_ErrorText(err, text, sizeof(text));
    snap_interop::LogJSON("error", "fatal_error", fixture.id,
                          std::string("\"error\": \"Srv_SetParam port: ") +
                              text + "\"");
    Srv_Destroy(&server);
    return 1;
  }

  if (fixture.pdu_size > 0) {
    int32_t pdu = fixture.pdu_size;
    err = Srv_SetParam(server, p_i32_PDURequest, &pdu);
    if (err != 0) {
      snap_interop::LogJSON(
          "warn", "fixture_loaded", fixture.id,
          "\"warning\": \"PDU size param not applied\", \"code\": " +
              std::to_string(err));
    }
  }

  for (auto& area : fixture.areas) {
    const int code = AreaCodeFor(area.type);
    if (code < 0) {
      snap_interop::LogJSON("error", "fatal_error", fixture.id,
                            "\"error\": \"unsupported area type " + area.type +
                                "\"");
      Srv_Destroy(&server);
      return 1;
    }
    word index = 0;
    if (area.type == "db") {
      if (area.number < 1 || area.number > 65535) {
        snap_interop::LogJSON("error", "fatal_error", fixture.id,
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
      snap_interop::LogJSON("error", "fatal_error", fixture.id,
                            std::string("\"error\": \"Srv_RegisterArea: ") +
                                text + "\"");
      Srv_Destroy(&server);
      return 1;
    }
  }

  std::signal(SIGTERM, OnSignal);
  std::signal(SIGINT, OnSignal);

  snap_interop::LogJSON(
      "info", "server_starting", fixture.id,
      "\"listen_address\": \"" + listen_addr +
          "\", \"port\": " + std::to_string(port));

  err = Srv_StartTo(server, listen_addr.c_str());
  if (err != 0) {
    char text[256];
    Srv_ErrorText(err, text, sizeof(text));
    snap_interop::LogJSON("error", "fatal_error", fixture.id,
                          std::string("\"error\": \"Srv_StartTo: ") + text +
                              "\"");
    Srv_Destroy(&server);
    return 1;
  }

  try {
    snap_interop::WriteStateSnapshot(state_path, fixture, "ready", counters);
  } catch (const std::exception& ex) {
    snap_interop::LogJSON("error", "fatal_error", fixture.id,
                          std::string("\"error\": \"") + ex.what() + "\"");
    Srv_Stop(server);
    Srv_Destroy(&server);
    return 1;
  }

  snap_interop::LogJSON("info", "server_ready", fixture.id,
                        "\"port\": " + std::to_string(port));

  while (!g_stop.load()) {
    // Periodically refresh diagnostic snapshot so writes are visible.
    try {
      snap_interop::WriteStateSnapshot(state_path, fixture, "ready", counters);
    } catch (...) {
      // Best-effort diagnostics must not kill the server.
    }
    std::this_thread::sleep_for(std::chrono::milliseconds(500));
  }

  snap_interop::LogJSON("info", "server_stopping", fixture.id, "{}");
  Srv_Stop(server);
  try {
    snap_interop::WriteStateSnapshot(state_path, fixture, "stopped", counters);
  } catch (...) {
  }
  Srv_Destroy(&server);
  snap_interop::LogJSON("info", "server_stopped", fixture.id, "{}");
  return 0;
}
