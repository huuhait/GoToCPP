// C++ equivalent of examples/raylib/main.gp (its smoke mode), to check output.txt.
#include <raylib.h>
#include <cstdio>
#include <cstdlib>
#include <iostream>
#include <string>
constexpr int width = 800, height = 450, smokeFrames = 3;
struct Ball {
    float X, Y, DX, DY, R;
    void Step() {
        X += DX;
        Y += DY;
        if (X < R || X > width - R)
            DX = -DX;
        if (Y < R || Y > height - R)
            DY = -DY;
    }
};
static long run(Ball& ball, bool smoke) {
    SetTraceLogLevel(LOG_ERROR);
    if (smoke)
        SetConfigFlags(FLAG_WINDOW_HIDDEN);
    InitWindow(width, height, "gocpp + raylib");
    if (!IsWindowReady()) {
        std::cout << "raylib: no window (is there a display?)\n";
        return 0;
    }
    std::cout << "window: " << GetScreenWidth() << " x " << GetScreenHeight() << "\n";
    SetTargetFPS(60);
    long frames = 0;
    bool paused = false;
    while (!WindowShouldClose() && !(smoke && frames == smokeFrames)) {
        if (IsKeyPressed(KEY_SPACE))
            paused = !paused;
        if (!paused)
            ball.Step();
        BeginDrawing();
        ClearBackground(RAYWHITE);
        DrawCircleV(Vector2{ball.X, ball.Y}, ball.R, MAROON);
        std::string text = "frame " + std::to_string(frames);
        DrawText(text.c_str(), 10, 10, 20, DARKGRAY);
        DrawText("Space: pause, Esc: quit", 10, 40, 20, LIGHTGRAY);
        EndDrawing();
        frames++;
    }
    CloseWindow();
    return frames;
}
int main() {
    bool smoke = std::getenv("GOCPP_SMOKE") != nullptr;
    Ball ball{width / 2, height / 2, 4, 3, 20};
    long frames = run(ball, smoke);
    std::cout << "frames: " << frames << " ball at: " << int(ball.X) << " " << int(ball.Y) << "\n";
    std::cout << "window closed\n";
}
