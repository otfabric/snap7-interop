#pragma once

#include "fixture.hpp"

#include <string>

namespace snap7_interop {

struct RuntimeCounters {
  uint64_t accepted_writes = 0;
  uint64_t rejected_accesses = 0;
};

void WriteStateSnapshot(const std::string& path,
                        const CompiledFixture& fixture,
                        const std::string& status,
                        const RuntimeCounters& counters);

}  // namespace snap7_interop
