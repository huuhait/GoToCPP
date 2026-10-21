// A field can name a struct that nothing declared before: the name belongs to
// the namespace around the class, not to the class (sqlite3.h does this in
// sqlite3_file, which points to sqlite3_io_methods).
typedef struct Holder Holder;

struct Holder {
	struct Later *next;
	struct Opaque *handle;
};

struct Later {
	int value;
};

namespace ns {
struct Box {
	struct Inside *inner;
};

struct Inside {
	int z;
};
}  // namespace ns

// A struct named two classes deep still belongs to the namespace.
struct Outer {
	struct Nested {
		struct Deep *deeper;
	} nested;
};

struct Deep {
	int depth;
};
