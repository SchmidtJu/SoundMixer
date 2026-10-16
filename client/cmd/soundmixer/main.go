//go:build windows

package main

import (
	"context"

	"soundmixer/internal/audio"
	"soundmixer/internal/autostart"
	"soundmixer/internal/config"
	"soundmixer/internal/instance"
	"soundmixer/internal/logx"
	"soundmixer/internal/serialconn"
	"soundmixer/internal/ui"
)

func main() {
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
	go ctrl.Run()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := ui.New(store, ctrl)
	go serialconn.Run(ctx, store, ctrl, app.SetStatus)

	if err := app.Run(); err != nil {
		logx.Printf("oberfläche: %v", err)
	}
	cancel()
	ctrl.Stop()
}
