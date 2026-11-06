#include "fixture.hpp"

#include <fstream>
#include <stdexcept>

#include <nlohmann/json.hpp>

namespace snap7_interop {
namespace {

std::vector<uint8_t> DecodeBase64(const std::string& in) {
  static const std::string kAlphabet =
      "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
  std::vector<int> T(256, -1);
  for (int i = 0; i < 64; ++i) {
    T[static_cast<unsigned char>(kAlphabet[i])] = i;
  }

  std::vector<uint8_t> out;
  out.reserve(in.size() * 3 / 4);
  int val = 0;
  int valb = -8;
  for (unsigned char c : in) {
    if (c == '=' || c == '\n' || c == '\r' || c == ' ') {
      continue;
    }
    if (T[c] == -1) {
      throw std::runtime_error("invalid base64 in compiled fixture");
    }
    val = (val << 6) + T[c];
    valb += 6;
    if (valb >= 0) {
      out.push_back(static_cast<uint8_t>((val >> valb) & 0xFF));
      valb -= 8;
    }
  }
  return out;
}

}  // namespace

CompiledFixture LoadCompiledFixture(const std::string& path) {
  std::ifstream in(path);
  if (!in) {
    throw std::runtime_error("cannot open fixture: " + path);
  }
  nlohmann::json doc;
  in >> doc;

  if (doc.value("format", "") != "snap-interop-compiled-fixture") {
    throw std::runtime_error("unsupported compiled fixture format");
  }
  if (doc.value("formatVersion", "") != "1.0.0") {
    throw std::runtime_error("unsupported compiled fixture formatVersion");
  }

  CompiledFixture fx;
  const auto& meta = doc.at("fixture");
  fx.id = meta.at("id").get<std::string>();
  fx.revision = meta.value("revision", 1);
  fx.source_sha256 = meta.value("sourceSha256", "");
  fx.compiled_sha256 = meta.value("compiledSha256", "");

  const auto& server = doc.at("server");
  fx.rack = server.value("rack", 0);
  fx.slot = server.value("slot", 2);
  fx.pdu_size = server.value("pduSize", 480);

  for (const auto& area : doc.at("areas")) {
    AreaBuffer ab;
    ab.type = area.at("type").get<std::string>();
    ab.size = area.at("size").get<int>();
    ab.access = area.value("access", "read-write");
    if (ab.type == "db") {
      ab.number = area.at("number").get<int>();
    }
    if (ab.size <= 0 || ab.size > 1024 * 1024) {
      throw std::runtime_error("area size out of allowed range");
    }
    ab.data = DecodeBase64(area.at("dataBase64").get<std::string>());
    if (static_cast<int>(ab.data.size()) != ab.size) {
      throw std::runtime_error("area dataBase64 length mismatch for type=" +
                               ab.type);
    }
    // Ensure capacity is exact and stable; Snap7 keeps this pointer.
    ab.data.shrink_to_fit();
    fx.areas.push_back(std::move(ab));
  }

  if (fx.areas.empty()) {
    throw std::runtime_error("compiled fixture has no areas");
  }
  return fx;
}

}  // namespace snap7_interop
