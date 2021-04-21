package params

import (
	"github.com/anyswap/CrossChain-Bridge/log"
	"github.com/fsnotify/fsnotify"
)

// WatchAndReloadScanConfig reload scan config if modified
func WatchAndReloadScanConfig() {
	log.Info("start job of watch and reload config")
	watch, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error("fsnotify.NewWatcher failed", "err", err)
		return
	}
	defer func() {
		err = watch.Close()
		if err != nil {
			log.Error("fsnotify close watcher failed", "err", err)
		}
	}()

	err = watch.Add(configFile)
	if err != nil {
		log.Error("watch.Add config file failed", "err", err)
		return
	}
	log.Info("watch.Add config file success", "configFile", configFile)

	ops := []fsnotify.Op{
		fsnotify.Create,
		fsnotify.Write,
	}

	for {
		select {
		case ev, ok := <-watch.Events:
			if !ok {
				continue
			}
			log.Info("fsnotify watch event", "event", ev)
			for _, op := range ops {
				if ev.Op&op == op {
					ReloadConfig()
					break
				}
			}
		case werr, ok := <-watch.Errors:
			if !ok {
				continue
			}
			log.Warn("fsnotify watch error", "err", werr)
		}
	}
}
