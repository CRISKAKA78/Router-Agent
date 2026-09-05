#ifndef RMP_SHA256_H
#define RMP_SHA256_H
#include <cstddef>
#include <cstdint>
#include <string>
namespace rmp {
class Sha256 {
  public:
    Sha256();
    void Update(const void *, std::size_t);
    std::string Finish();

  private:
    void Block(const std::uint8_t *);
    std::uint32_t h_[8];
    std::uint8_t buf_[64];
    std::size_t used_;
    std::uint64_t bytes_;
};
} // namespace rmp
#endif
