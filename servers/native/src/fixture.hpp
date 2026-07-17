#pragma once

#include <cstdint>
#include <string>
#include <vector>

namespace snap_interop {

struct AreaBuffer {
  std::string type;
  int number = 0;  // meaningful for DB
  int size = 0;
  std::string access;
  std::vector<uint8_t> data;  // stable backing store for Snap7
};

struct CompiledFixture {
  std::string id;
  int revision = 1;
  std::string source_sha256;
  std::string compiled_sha256;
  int rack = 0;
  int slot = 2;
  int pdu_size = 480;
  std::vector<AreaBuffer> areas;
};

// Load and validate a compiled fixture JSON document.
CompiledFixture LoadCompiledFixture(const std::string& path);

}  // namespace snap_interop
