// Declarations that the header marks deprecated, with and without a message.
__attribute__((deprecated)) int old_plain(void);
__attribute__((deprecated("use new_one"))) int old_message(int x);
[[deprecated("use Fresh")]] int old_cxx_attribute(void);
struct __attribute__((deprecated("gone"))) OldStruct { int a; };
typedef int OldAlias __attribute__((deprecated("alias")));
enum __attribute__((deprecated)) OldEnum { OLD_A };
__attribute__((deprecated("variable"))) extern int old_var;
__attribute__((deprecated("two\nlines"))) int old_multiline(void);
typedef void OldVoid __attribute__((deprecated("void typedef")));
enum Mixed { MIXED_OK, MIXED_OLD [[deprecated("use MIXED_OK")]], MIXED_PLAIN_OLD __attribute__((deprecated)) };
namespace ns {
[[deprecated("namespaced")]] int old_in_ns(void);
struct [[deprecated("namespaced type")]] OldInNs { int a; };
struct Holder {
	struct [[deprecated("nested type")]] OldNested { int a; };
};
struct Incomplete;
}
struct [[deprecated("incomplete")]] OldIncomplete;
int current(void);
struct Current { int a; };
