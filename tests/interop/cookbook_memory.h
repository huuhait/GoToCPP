// A small C library for cookbook_memory.gp: a linked list that C allocates and frees, and a
// buffer that C allocates.
#pragma once

#include <stdlib.h>

typedef struct node {
    int value;
    struct node* next;
} node;

static inline node* list_make(int n) { // C allocates: C must free (list_free)
    node* head = NULL;
    for (int i = n; i > 0; i--) {
        node* x = (node*)malloc(sizeof(node));
        x->value = i;
        x->next = head;
        head = x;
    }
    return head;
}

static inline void list_free(node* head) {
    while (head != NULL) {
        node* next = head->next;
        free(head);
        head = next;
    }
}

static inline int list_sum(const node* head) {
    int s = 0;
    for (; head != NULL; head = head->next) {
        s += head->value;
    }
    return s;
}

static inline int* ints_make(int n) {
    int* p = (int*)malloc(sizeof(int) * (size_t)n);
    for (int i = 0; i < n; i++) {
        p[i] = i * i;
    }
    return p;
}
