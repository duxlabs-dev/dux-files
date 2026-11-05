package main

import (
	"github.com/duxlabs-dev/dux-files/core/internal/fileops"
	"github.com/duxlabs-dev/dux-files/core/internal/ipc"
	"github.com/duxlabs-dev/dux-files/core/internal/mimeapps"
	"github.com/duxlabs-dev/dux-files/core/internal/sandbox"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/log"
)

func noServices(*ipc.EventBus) (ipc.Deps, func()) { return ipc.Deps{}, func() {} }

func appServices(*ipc.EventBus) (ipc.Deps, func()) {
	return ipc.Deps{Apps: mimeapps.NewService()}, func() {}
}

func fileServices(bus *ipc.EventBus) (ipc.Deps, func()) {
	apps := mimeapps.NewService()
	fileService := files.NewService(bus, cacheHome(), apps.Detector())
	return ipc.Deps{Apps: apps, Files: fileService, FileOps: fileops.NewManager(bus, fileService)}, fileService.Close
}

func searchServices(bus *ipc.EventBus) (ipc.Deps, func()) {
	fileService := files.NewService(bus, cacheHome(), mimeapps.NewService().Detector())
	deviceService := startDevices(bus, log.Debugf)
	searchService := newSearch(bus, fileService, deviceService, configHome(), sandbox.Current())
	return ipc.Deps{Files: fileService, Search: searchService}, func() {
		searchService.Close()
		deviceService.Close()
		fileService.Close()
	}
}

func deviceServices(bus *ipc.EventBus) (ipc.Deps, func()) {
	deviceService := startDevices(bus, log.Debugf)
	return ipc.Deps{Devices: deviceService}, deviceService.Close
}
