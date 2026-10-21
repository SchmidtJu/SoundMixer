//go:build windows

package main

import (
	"context"
	"flag"

	"soundmixer/internal/audio"
	"soundmixer/internal/autostart"
	"soundmixer/internal/config"
	"soundmixer/internal/instance"
	"soundmixer/internal/logx"
	"soundmixer/internal/serialconn"
	"soundmixer/internal/ui"
)

func main() {
	showSettings := flag.Bool("settings", false, "Open the settings window on startup")
	flag.Parse()
	if !instance.Acquire() {
		instance.AlertAlreadyRunning()
		return
	}

	dir, err := config.Dir()
	if err != nil {
		return
	}
	logx.Init(dir)

	store, err := config.Load()
	if err != nil {
		logx.Printf("config: %v", err)
		return
	}
	logx.SetEnabled(store.Get().Logging)
	if err := autostart.Set(store.Get().Autostart); err != nil {
		logx.Printf("autostart: %v", err)
	}

	ctrl := audio.New()
	ctrl.SetUpdateInterval(store.Get().UpdateIntervalMs)
	go ctrl.Run()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := ui.New(store, ctrl)
	go serialconn.Run(ctx, store, ctrl, app.SetStatus, app.SetSliderValues)

	if err := app.Run(*showSettings); err != nil {
		logx.Printf("ui: %v", err)
	}
	cancel()
	ctrl.Stop()
}
