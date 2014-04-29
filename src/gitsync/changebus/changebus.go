package changebus

import (
	"fmt"
	"gitsync"
	"sync"
)

// ChangeListener represents a single sink for change data from a ChangeBus
type ChangeListener chan gitsync.GitChange

// ChangeBus broadcasts any gitsync.GitChange sent to it to all listeners. It is
// a basic pub-sub channel.
type ChangeBus interface {
	// Publish will send a change to all listeners
	Publish(cb gitsync.GitChange) error

	// GetPublishChannel returns a channel that can be used to send changes on
	// this bus.
	GetPublishChannel() chan gitsync.GitChange

	// RegisterListener adds a new listener to this bus. Listeners should expect
	// to be closed to indicate the ChangeBus is no longer being used.
	// Note: listeners should not close their channels themselves.
	RegisterListener(listener ChangeListener)

	// GetNewListener returns a channel that can be listened on. It should not be
	// closed.
	GetNewListener() (listener ChangeListener)

	// Close all channels to signal the bus is no longer to be used
	Close()
}

// changeBus implements ChangeBus above.
type changeBus struct {
	sync.RWMutex                        // sync changes to the listeners list
	listeners    []ChangeListener       // channels to send published changes to
	publish      chan gitsync.GitChange // channel to receive changes to publish
}

// New returns an instance of a ChangeBus with publish queue length size and
// that will report any errors on errors.
func New(size int, errors chan error) ChangeBus {
	cb := &changeBus{
		listeners: make([]ChangeListener, 0),
		publish:   make(chan gitsync.GitChange, size),
	}

	go cb.processChanges(errors)

	return cb
}

func (cb *changeBus) shareChange(change gitsync.GitChange, errors chan error) {
	cb.RLock()
	defer cb.RUnlock()

	for _, listener := range cb.listeners {
		select {
		case listener <- change:
			// Good. no-op
		default:
			// try to send the error for reporting, dropping them if errors is nil
			if errors != nil {
				select {
				case errors <- fmt.Errorf("Cannot send to %v", listener):
				default:
					// silently drop errors sending to errors
				}
			}
		}
	}
}

func (cb *changeBus) processChanges(errors chan error) {
	// loop over all changes sent on publish
	for change := range cb.publish {
		// share them with all listeners
		cb.shareChange(change, errors)
	}

	// we get here only on a close of cb.publish
	for _, listener := range cb.listeners {
		close(listener)
	}
}

func (cb *changeBus) Close() {
	close(cb.publish)
}

func (cb *changeBus) RegisterListener(listener ChangeListener) {
	cb.Lock()
	defer cb.Unlock()

	cb.listeners = append(cb.listeners, listener)
}

func (cb *changeBus) GetNewListener() (listener ChangeListener) {
	ret := make(chan gitsync.GitChange)
	cb.RegisterListener(ret)
	return ret
}

func (cb *changeBus) Publish(change gitsync.GitChange) (err error) {
	select {
	case cb.publish <- change:
		return nil
	default:
		return fmt.Errorf("Bus full. Dropped new change.")
	}
}

func (cb *changeBus) GetPublishChannel() chan gitsync.GitChange {
	return cb.publish
}
