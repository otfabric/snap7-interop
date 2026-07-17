#include "state.hpp"

#include <chrono>
#include <cstdio>
#include <filesystem>
#include <fstream>
#include <stdexcept>

#include <nlohmann/json.hpp>

namespace snap_interop {
namespace {

std::string UTCTimestamp() {
  using clock = std::chrono::system_clock;
  const auto now = clock::now();
  const auto secs = clock::to_time_t(now);
  std::tm tm{};
  gmtime_r(&secs, &tm);
  char buf[40];
  std::snprintf(buf, sizeof(buf), "%04d-%02d-%02dT%02d:%02d:%02dZ",
                tm.tm_year + 1900, tm.tm_mon + 1, tm.tm_mday, tm.tm_hour,
                tm.tm_min, tm.tm_sec);
  return buf;
}

std::string ToBase64(const std::vector<uint8_t>& data) {
  static const char* kAlphabet =
      "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
  std::string out;
  out.reserve(((data.size() + 2) / 3) * 4);
  size_t i = 0;
  while (i + 3 <= data.size()) {
    const uint32_t n = (uint32_t(data[i]) << 16) | (uint32_t(data[i + 1]) << 8) |
                       uint32_t(data[i + 2]);
    out.push_back(kAlphabet[(n >> 18) & 63]);
    out.push_back(kAlphabet[(n >> 12) & 63]);
    out.push_back(kAlphabet[(n >> 6) & 63]);
    out.push_back(kAlphabet[n & 63]);
    i += 3;
  }
  if (i < data.size()) {
    uint32_t n = uint32_t(data[i]) << 16;
    out.push_back(kAlphabet[(n >> 18) & 63]);
    if (i + 1 < data.size()) {
      n |= uint32_t(data[i + 1]) << 8;
      out.push_back(kAlphabet[(n >> 12) & 63]);
      out.push_back(kAlphabet[(n >> 6) & 63]);
      out.push_back('=');
    } else {
      out.push_back(kAlphabet[(n >> 12) & 63]);
      out.push_back('=');
      out.push_back('=');
    }
  }
  return out;
}

}  // namespace

void WriteStateSnapshot(const std::string& path,
                        const CompiledFixture& fixture,
                        const std::string& status,
                        const RuntimeCounters& counters) {
  nlohmann::json areas = nlohmann::json::array();
  for (const auto& a : fixture.areas) {
    nlohmann::json item = {
        {"type", a.type},
        {"size", a.size},
        {"access", a.access},
        {"dataBase64", ToBase64(a.data)},
    };
    if (a.type == "db") {
      item["number"] = a.number;
    }
    areas.push_back(std::move(item));
  }

  nlohmann::json doc = {
      {"fixture",
       {{"id", fixture.id},
        {"revision", fixture.revision},
        {"compiledSha256", fixture.compiled_sha256}}},
      {"implementation",
       {{"name", "native"}, {"adapterVersion", "0.1.0"}}},
      {"serverStatus", status},
      {"acceptedWriteCount", counters.accepted_writes},
      {"rejectedAccessCount", counters.rejected_accesses},
      {"lastUpdateTime", UTCTimestamp()},
      {"areas", std::move(areas)},
  };

  const std::filesystem::path p(path);
  if (p.has_parent_path()) {
    std::filesystem::create_directories(p.parent_path());
  }
  const auto tmp = p.string() + ".tmp";
  {
    std::ofstream out(tmp, std::ios::trunc);
    if (!out) {
      throw std::runtime_error("cannot write state snapshot temp file: " + tmp);
    }
    out << doc.dump(2) << '\n';
    if (!out) {
      throw std::runtime_error("failed while writing state snapshot: " + tmp);
    }
  }
  std::error_code ec;
  std::filesystem::rename(tmp, p, ec);
  if (ec) {
    throw std::runtime_error("cannot rename state snapshot: " + ec.message());
  }
}

}  // namespace snap_interop
