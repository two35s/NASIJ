package framework

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func fm(path, content string) FileMeta {
	return FileMeta{Path: path, Content: content}
}

func TestDetectReact(t *testing.T) {
	files := []FileMeta{
		fm("package.json", `{"dependencies":{"react":"18.2.0","react-dom":"18.2.0"}}`),
		fm("src/App.jsx", `import React from 'react';`),
		fm("src/index.js", `import ReactDOM from 'react-dom'; ReactDOM.render(<App/>, root);`),
	}
	r := Detect(".", files)
	fw := find(r, "react")
	if fw == nil {
		t.Fatal("expected react to be detected")
	}
	if fw.Confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5, got %f", fw.Confidence)
	}
	t.Logf("react confidence: %f, signals: %d", fw.Confidence, len(fw.Signals))
}

func TestDetectReactNoDeps(t *testing.T) {
	files := []FileMeta{
		fm("src/App.tsx", `import React, { useState } from 'react';`),
		fm("src/index.tsx", `<App/>`),
	}
	r := Detect(".", files)
	fw := find(r, "react")
	if fw == nil {
		t.Fatal("expected react to be detected from source")
	}
}

func TestDetectNext(t *testing.T) {
	files := []FileMeta{
		fm("package.json", `{"dependencies":{"next":"14.0.0","react":"18.2.0","react-dom":"18.2.0"}}`),
		fm("next.config.js", `module.exports = {};`),
		fm("pages/index.js", `export default function Home() { return <div/>; }`),
	}
	r := Detect(".", files)
	fw := find(r, "next")
	if fw == nil {
		t.Fatal("expected next to be detected")
	}
	if fw.Confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5, got %f", fw.Confidence)
	}
}

func TestDetectAngular(t *testing.T) {
	files := []FileMeta{
		fm("package.json", `{"dependencies":{"@angular/core":"17.0.0","@angular/platform-browser":"17.0.0"}}`),
		fm("angular.json", `{}`),
		fm("src/app/app.component.ts", `
			import { Component } from '@angular/core';
			@Component({selector: 'app-root', template: '<h1>Hello</h1>'})
			export class AppComponent {}
		`),
	}
	r := Detect(".", files)
	fw := find(r, "angular")
	if fw == nil {
		t.Fatal("expected angular to be detected")
	}
	if fw.Confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5, got %f", fw.Confidence)
	}
}

func TestDetectVue(t *testing.T) {
	files := []FileMeta{
		fm("package.json", `{"dependencies":{"vue":"3.4.0"}}`),
		fm("src/App.vue", `<template><div>{{ msg }}</div></template>`),
		fm("src/main.js", `import { createApp } from 'vue'; createApp(App).mount('#app');`),
	}
	r := Detect(".", files)
	fw := find(r, "vue")
	if fw == nil {
		t.Fatal("expected vue to be detected")
	}
	if fw.Confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5, got %f", fw.Confidence)
	}
}

func TestDetectNuxt(t *testing.T) {
	files := []FileMeta{
		fm("package.json", `{"dependencies":{"nuxt":"3.8.0","vue":"3.4.0"}}`),
		fm("nuxt.config.ts", `export default defineNuxtConfig({})`),
		fm("pages/index.vue", `<template><h1>Home</h1></template>`),
	}
	r := Detect(".", files)
	fw := find(r, "nuxt")
	if fw == nil {
		t.Fatal("expected nuxt to be detected")
	}
	if fw.Confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5, got %f", fw.Confidence)
	}
}

func TestDetectAstro(t *testing.T) {
	files := []FileMeta{
		fm("package.json", `{"dependencies":{"astro":"4.0.0"}}`),
		fm("astro.config.mjs", `import { defineConfig } from 'astro/config';`),
		fm("src/pages/index.astro", `---
import Layout from '../layouts/Layout.astro';
---
<Layout><h1>Hello</h1></Layout>`),
	}
	r := Detect(".", files)
	fw := find(r, "astro")
	if fw == nil {
		t.Fatal("expected astro to be detected")
	}
	if fw.Confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5, got %f", fw.Confidence)
	}
}

func TestDetectRemix(t *testing.T) {
	files := []FileMeta{
		fm("package.json", `{"dependencies":{"@remix-run/react":"2.0.0","@remix-run/node":"2.0.0"}}`),
		fm("remix.config.js", `module.exports = {};`),
		fm("app/routes/_index.tsx", `
			import { useLoaderData } from '@remix-run/react';
			export async function loader() { return {}; }
		`),
	}
	r := Detect(".", files)
	fw := find(r, "remix")
	if fw == nil {
		t.Fatal("expected remix to be detected")
	}
	if fw.Confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5, got %f", fw.Confidence)
	}
}

func TestDetectSvelte(t *testing.T) {
	files := []FileMeta{
		fm("package.json", `{"dependencies":{"svelte":"4.0.0"}}`),
		fm("src/App.svelte", `<script>export let name;</script>`),
		fm("src/main.js", `import App from './App.svelte';`),
	}
	r := Detect(".", files)
	fw := find(r, "svelte")
	if fw == nil {
		t.Fatal("expected svelte to be detected")
	}
	if fw.Confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5, got %f", fw.Confidence)
	}
}

func TestDetectWebpack(t *testing.T) {
	files := []FileMeta{
		fm("package.json", `{"devDependencies":{"webpack":"5.89.0","webpack-cli":"5.1.0"}}`),
		fm("webpack.config.js", `module.exports = { entry: './src/index.js' };`),
		fm("src/index.js", `__webpack_public_path__ = '/';`),
	}
	r := Detect(".", files)
	fw := find(r, "webpack")
	if fw == nil {
		t.Fatal("expected webpack to be detected")
	}
	if fw.Confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5, got %f", fw.Confidence)
	}
}

func TestDetectVite(t *testing.T) {
	files := []FileMeta{
		fm("package.json", `{"devDependencies":{"vite":"5.0.0"}}`),
		fm("vite.config.ts", `import { defineConfig } from 'vite';`),
		fm("src/main.ts", `import.meta.env.VITE_API_URL;`),
	}
	r := Detect(".", files)
	fw := find(r, "vite")
	if fw == nil {
		t.Fatal("expected vite to be detected")
	}
	if fw.Confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5, got %f", fw.Confidence)
	}
}

func TestDetectMultiple(t *testing.T) {
	// React + Vite + Webpack
	files := []FileMeta{
		fm("package.json", `{
			"dependencies": {"react":"18.2.0","react-dom":"18.2.0"},
			"devDependencies": {"vite":"5.0.0","@vitejs/plugin-react":"4.0.0"}
		}`),
		fm("vite.config.ts", `import { defineConfig } from 'vite';`),
		fm("src/main.tsx", `import React from 'react'; import.meta.env.VITE_API_URL;`),
	}
	r := Detect(".", files)
	react := find(r, "react")
	vite := find(r, "vite")
	if react == nil {
		t.Error("expected react in multi-detection")
	}
	if vite == nil {
		t.Error("expected vite in multi-detection")
	}
	t.Logf("detected %d frameworks", len(r.Frameworks))
	for _, fw := range r.Frameworks {
		t.Logf("  %s: confidence=%.2f, signals=%d", fw.Name, fw.Confidence, len(fw.Signals))
	}
}

func TestDetectNone(t *testing.T) {
	files := []FileMeta{
		fm("README.md", "# Plain project"),
		fm("src/index.js", `console.log("hello");`),
	}
	r := Detect(".", files)
	if len(r.Frameworks) != 0 {
		t.Errorf("expected 0 frameworks, got %d: %v", len(r.Frameworks), r.Frameworks)
	}
}

func TestDetectFromPaths(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"react":"18.2.0"}}`), 0644)
	os.WriteFile(filepath.Join(dir, "index.jsx"), []byte(`import React from 'react';`), 0644)
	r := DetectFromPaths(dir, 1<<20)
	react := find(r, "react")
	if react == nil {
		t.Fatal("expected react from path detection")
	}
}

func TestDetectFromPathsNoMatch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main`), 0644)
	r := DetectFromPaths(dir, 1<<20)
	if len(r.Frameworks) != 0 {
		t.Errorf("expected 0 frameworks, got %d", len(r.Frameworks))
	}
}

func TestKnownFrameworks(t *testing.T) {
	expected := map[string]bool{
		"react": true, "next": true, "angular": true, "vue": true,
		"nuxt": true, "astro": true, "remix": true, "svelte": true,
		"webpack": true, "vite": true,
	}
	if len(KnownFrameworks) != len(expected) {
		t.Errorf("expected %d known frameworks, got %d", len(expected), len(KnownFrameworks))
	}
	for _, name := range KnownFrameworks {
		if !expected[name] {
			t.Errorf("unexpected framework in KnownFrameworks: %s", name)
		}
	}
}

func TestDetectNextNoPkgJson(t *testing.T) {
	files := []FileMeta{
		fm("next.config.js", `module.exports = {};`),
		fm("pages/index.js", `export default function Page() { return null; }`),
	}
	r := Detect(".", files)
	fw := find(r, "next")
	if fw == nil {
		t.Fatal("expected next to be detected even without package.json")
	}
}

func TestDetectVueSourceOnly(t *testing.T) {
	files := []FileMeta{
		fm("src/App.vue", `<template><div v-if="ok">hello</div></template>`),
		fm("main.js", `import { createApp } from 'vue';`),
	}
	r := Detect(".", files)
	fw := find(r, "vue")
	if fw == nil {
		t.Fatal("expected vue from source+ext")
	}
}

func TestDetectAngularJS(t *testing.T) {
	files := []FileMeta{
		fm("index.html", `<html ng-app="myApp"><body ng-controller="MainCtrl"></body></html>`),
	}
	r := Detect(".", files)
	fw := find(r, "angular")
	if fw == nil {
		t.Fatal("expected angular from AngularJS directives")
	}
}

func TestDetectSvelteSource(t *testing.T) {
	files := []FileMeta{
		fm("src/Counter.svelte", `<script>let count = 0; $: doubled = count * 2;</script>
<button on:click={() => count++}>{count}</button>`),
	}
	r := Detect(".", files)
	fw := find(r, "svelte")
	if fw == nil {
		t.Fatal("expected svelte from source patterns")
	}
}

func TestDetectWebpackSource(t *testing.T) {
	files := []FileMeta{
		fm("src/index.js", `__webpack_require__("./module");`),
	}
	r := Detect(".", files)
	fw := find(r, "webpack")
	if fw == nil {
		t.Fatal("expected webpack from source")
	}
}

func TestDetectEmptyFiles(t *testing.T) {
	r := Detect(".", nil)
	if len(r.Frameworks) != 0 {
		t.Errorf("expected 0 frameworks for empty files, got %d", len(r.Frameworks))
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func find(r *Result, name string) *Framework {
	for i := range r.Frameworks {
		if r.Frameworks[i].Name == name {
			return &r.Frameworks[i]
		}
	}
	return nil
}
