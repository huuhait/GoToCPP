// C++ equivalent of examples/cpp-std/main.gp, to check its output.txt.
#include <algorithm>
#include <cstdint>
#include <filesystem>
#include <iostream>
#include <map>
#include <string>
#include <vector>
namespace fs = std::filesystem;
int main() {
    std::cout << std::boolalpha;
    auto nums = std::vector<int64_t>{5, 3, 8, 1};
    nums.push_back(4);
    std::sort(nums.begin(), nums.end());
    std::cout << "sorted: [";
    for (size_t i = 0; i < nums.size(); i++)
        std::cout << (i ? " " : "") << nums[i];
    std::cout << "] size: " << (int64_t)nums.size() << "\n";
    std::cout << "front: " << nums.front() << " back: " << nums.back() << " at(1): " << nums.at(1)
              << "\n";
    auto pos = std::find(nums.begin(), nums.end(), 5) - nums.begin();
    std::cout << "index of 5: " << (int64_t)pos << " max: " << std::max(nums.front(), nums.back())
              << "\n";
    // like gp::values: copy the references before the temporaries die
    auto [lo, hi] = std::pair<int, int>(std::minmax(3, 1));
    std::cout << "minmax: " << lo << " " << hi << "\n";
    auto ages = std::map<std::string, int64_t>{};
    ages["bob"] = 31;
    ages["alice"] = 29;
    ages["carol"] = 35;
    ages["bob"]++;
    for (auto&& [name, age] : ages)
        std::cout << name << " " << age << "\n";
    auto dave = ages["dave"];
    std::cout << "dave: " << dave << " size: " << (int64_t)ages.size()
              << " has eve: " << (ages.count("eve") == 1) << "\n";
    auto s = std::string("hello");
    s.append(", world");
    s += "!";
    std::cout << s << " " << (int64_t)s.size() << "\n";
    if (auto at = s.find("world"); at != std::string::npos)
        std::cout << "world at " << (int64_t)at << "\n";
    std::cout << s.substr(0, 5) + "?" << "\n";
    std::reverse(s.begin(), s.end());
    std::cout << s << "\n";
    std::string gs = "from gocpp";
    auto cs = std::string(gs);
    std::cout << (int64_t)cs.length() << " " << (cs == "from gocpp") << "\n";
    auto p = fs::path("/home/gopher/notes/todo.txt");
    std::cout << p.filename().string() << " " << p.stem().string() << " " << p.extension().string()
              << "\n";
    std::cout << p.parent_path().generic_string() << "\n";
    auto q = p.parent_path() / "archive" / "2026.txt";
    std::cout << q.generic_string() << "\n";
    p.replace_extension(".md");
    std::cout << p.filename().string() << "\n";
    std::cout << fs::path("a/./b/../c.txt").lexically_normal().generic_string() << "\n";
}
