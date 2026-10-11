#pragma once

namespace utils {
inline constexpr auto convert_to_percent = [](auto active, auto total) {
  if (total == 0)
    return 0.0;
  return (static_cast<double>(active) / static_cast<double>(total)) * 100.0;
};
} // namespace utils
