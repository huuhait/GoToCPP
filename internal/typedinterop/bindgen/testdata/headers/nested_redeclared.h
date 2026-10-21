// A struct whose forward declaration comes first (sqlite3.h declares
// sqlite3_index_info this way) and whose body declares types of its own.
typedef struct Table Table;

struct Table {
	int count;
	struct Column {
		int index;
		unsigned char flags;
	} *columns;
	enum Mode { MODE_READ, MODE_WRITE } mode;
	typedef int Handle;
	Handle handle;
};
