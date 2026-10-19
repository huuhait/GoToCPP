#pragma once
/// The speed of light in m/s.
#define PHYS_C 299792458

/// Adds two vectors.
///
/// Both are in meters.
double vec_add(double a, double b);

//! A body in space.
struct Body {
  double mass;
};

/** The gravitational constant. */
extern const double G;

int undocumented(int x);
