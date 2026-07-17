#pragma once

#include <string>

namespace snap_interop {

void LogJSON(const std::string& level,
             const std::string& event,
             const std::string& fixture_id,
             const std::string& fields_json_object = "{}");

}  // namespace snap_interop
