// Dear ImGui with GLFW and OpenGL 3: a window with a few widgets (text, a
// slider, a color editor, a checkbox that opens ImGui's own demo window, and a
// button with a counter). The library is not in this folder: gocpp.mod
// requires github.com/ocornut/imgui and holds the "cxx" recipe that builds it,
// and `gocpp get` downloads it. The widgets are called as ImGui.Begin,
// ImGui.SliderFloat, ... straight from gocpp code. See README.md for how to
// build and run it.
package main

import (
	"fmt"

	"github.com/ocornut/imgui/backends/imgui_impl_glfw.h"
	"github.com/ocornut/imgui/backends/imgui_impl_opengl3.h"
	"github.com/ocornut/imgui/imgui.h"

	"<GL/gl.h>"
	"<GLFW/glfw3.h>"
)

func main() {
	if glfw3.glfwInit() == 0 {
		fmt.Println("glfw: cannot start")
		return
	}
	defer glfw3.glfwTerminate()

	window := glfw3.glfwCreateWindow(960, 540, "gocpp + Dear ImGui", nil, nil)
	if window == nil {
		fmt.Println("glfw: no window (is there a display?)")
		return
	}
	glfw3.glfwMakeContextCurrent(window)
	glfw3.glfwSwapInterval(1)

	imgui.ImGui.CreateContext()
	imgui.ImGui.StyleColorsDark()
	imgui_impl_glfw.ImGui_ImplGlfw_InitForOpenGL(window, true)
	imgui_impl_opengl3.ImGui_ImplOpenGL3_Init("#version 130")

	speed := float32(0.5)
	var showDemo bool
	var clicks int
	clear := [3]float32{0.10, 0.12, 0.16}

	for glfw3.glfwWindowShouldClose(window) == 0 {
		glfw3.glfwPollEvents()
		imgui_impl_opengl3.ImGui_ImplOpenGL3_NewFrame()
		imgui_impl_glfw.ImGui_ImplGlfw_NewFrame()
		imgui.ImGui.NewFrame()

		imgui.ImGui.Begin("gocpp")
		imgui.ImGui.Text("This window is Dear ImGui, driven from gocpp.")
		imgui.ImGui.SliderFloat("speed", &speed, 0, 1)
		imgui.ImGui.ColorEdit3("background", &clear[0])
		imgui.ImGui.Checkbox("show the ImGui demo window", &showDemo)
		if imgui.ImGui.Button("click me") {
			clicks++
		}
		imgui.ImGui.SameLine()
		imgui.ImGui.Text("%s", fmt.Sprintf("clicked %d times", clicks))
		imgui.ImGui.End()
		if showDemo {
			imgui.ImGui.ShowDemoWindow(&showDemo)
		}

		imgui.ImGui.Render()
		var width, height int
		glfw3.glfwGetFramebufferSize(window, &width, &height)
		gl.glViewport(0, 0, width, height)
		gl.glClearColor(clear[0], clear[1], clear[2], 1)
		gl.glClear(gl.GL_COLOR_BUFFER_BIT)
		imgui_impl_opengl3.ImGui_ImplOpenGL3_RenderDrawData(imgui.ImGui.GetDrawData())
		glfw3.glfwSwapBuffers(window)
	}

	imgui_impl_opengl3.ImGui_ImplOpenGL3_Shutdown()
	imgui_impl_glfw.ImGui_ImplGlfw_Shutdown()
	imgui.ImGui.DestroyContext()
	fmt.Println("window closed, clicks:", clicks)
}
