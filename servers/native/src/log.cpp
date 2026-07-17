#include "log.hpp"

#include <chrono>
#include <cstdio>
#include <ctime>

namespace snap_interop {
namespace {

constexpr const char* kAdapterVersion = "0.1.0";

std::string UTCTimestamp() {
  using clock = std::chrono::system_clock;
  const auto now = clock::now();
  const auto secs = clock::to_time_t(now);
  const auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(
                      now.time_since_epoch()) %
                  1000;
  std::tm tm{};
  gmtime_r(&secs, &tm);
  char buf[64];
  std::snprintf(buf, sizeof(buf), "%04d-%02d-%02dT%02d:%02d:%02d.%03lldZ",
                tm.tm_year + 1900, tm.tm_mon + 1, tm.tm_mday, tm.tm_hour,
                tm.tm_min, tm.tm_sec, static_cast<long long>(ms.count()));
  return buf;
}

}  // namespace

void LogJSON(const std::string& level,
             const std::string& event,
             const std::string& fixture_id,
             const std::string& fields_json_object) {
  // fields_json_object must be a JSON object body without outer braces, or "{}".
  std::string fields = fields_json_object;
  if (fields.empty() || fields == "{}") {
    std::fprintf(stdout,
                 "{\"timestamp\": \"%s\", \"level\": \"%s\", "
                 "\"implementation\": \"native\", \"event\": \"%s\", "
                 "\"fixture_id\": \"%s\", \"adapter_version\": \"%s\"}\n",
                 UTCTimestamp().c_str(), level.c_str(), event.c_str(),
                 fixture_id.c_str(), kAdapterVersion);
  } else {
    // Strip surrounding braces if present.
    if (fields.front() == '{' && fields.back() == '}') {
      fields = fields.substr(1, fields.size() - 2);
    }
    std::fprintf(stdout,
                 "{\"timestamp\": \"%s\", \"level\": \"%s\", "
                 "\"implementation\": \"native\", \"event\": \"%s\", "
                 "\"fixture_id\": \"%s\", \"adapter_version\": \"%s\", %s}\n",
                 UTCTimestamp().c_str(), level.c_str(), event.c_str(),
                 fixture_id.c_str(), kAdapterVersion, fields.c_str());
  }
  std::fflush(stdout);
}

}  // namespace snap_interop
