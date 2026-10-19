// The native function of package maps (gocpp/maps.hpp, step 7.10).
#include <gocpp/rt.hpp>

namespace gp::maps {

gp::any clone(const gp::any& m) {
    const type_desc* t = m.gp_type();
    if (t == nullptr || t->gp_ops == nullptr || t->gp_ops->gp_map_clone == nullptr) {
        gp::panic(gp::string::gp_copy("maps.Clone: the map type can't be copied"));
    }
    return gp::any(gp::iface(t->gp_ops->gp_map_clone(m.gp_data().get()), t, m.gp_tab()));
}

} // namespace gp::maps
