#pragma once
#include "part_a.h"
typedef struct PartB { PartA base; int b; } PartB;
int part_b(PartB *p);
#define PART_B_MAGIC 0xB
