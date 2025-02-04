package mocks

import (
	"sync"

	"github.com/Anth80/sarama"
)

// SyncProducer implements sarama's SyncProducer interface for testing purposes.
// Before you can use it, you have to set expectations on the mock SyncProducer
// to tell it how to handle calls to SendMessage, so you can easily test success
// and failure scenarios.
type SyncProducer struct {
	l            sync.Mutex
	t            ErrorReporter
	expectations []*producerExpectation
	lastOffset   int64
}

// NewSyncProducer instantiates a new SyncProducer mock. The t argument should
// be the *testing.T instance of your test method. An error will be written to it if
// an expectation is violated. The config argument is currently unused, but is
// maintained to be compatible with the async Producer.
func NewSyncProducer(t ErrorReporter, config *sarama.Config) *SyncProducer {
	return &SyncProducer{
		t:            t,
		expectations: make([]*producerExpectation, 0),
	}
}

////////////////////////////////////////////////
// Implement SyncProducer interface
////////////////////////////////////////////////

// SendMessage corresponds with the SendMessage method of sarama's SyncProducer implementation.
// You have to set expectations on the mock producer before calling SendMessage, so it knows
// how to handle them. You can set a function in each expectation so that the message value
// checked by this function and an error is returned if the match fails.
// If there is no more remaining expectation when SendMessage is called,
// the mock producer will write an error to the test state object.
func (sp *SyncProducer) SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error) {
	sp.l.Lock()
	defer sp.l.Unlock()

	if len(sp.expectations) > 0 {
		expectation := sp.expectations[0]
		sp.expectations = sp.expectations[1:]
		if expectation.CheckFunction != nil {
			val, err := msg.Value.Encode()
			if err != nil {
				sp.t.Errorf("Input message encoding failed: %s", err.Error())
				return -1, -1, err
			}

			errCheck := expectation.CheckFunction(val)
			if errCheck != nil {
				sp.t.Errorf("Check function returned an error: %s", errCheck.Error())
				return -1, -1, errCheck
			}
		}
		if expectation.Result == errProduceSuccess {
			sp.lastOffset++
			msg.Offset = sp.lastOffset
			return 0, msg.Offset, nil
		}
		return -1, -1, expectation.Result
	}
	sp.t.Errorf("No more expectation set on this mock producer to handle the input message.")
	return -1, -1, errOutOfExpectations
}

// SendMessages corresponds with the SendMessages method of sarama's SyncProducer implementation.
// You have to set expectations on the mock producer before calling SendMessages, so it knows
// how to handle them. If there is no more remaining expectations when SendMessages is called,
// the mock producer will write an error to the test state object.
func (sp *SyncProducer) SendMessages(msgs []*sarama.ProducerMessage) error {
	sp.l.Lock()
	defer sp.l.Unlock()

	if len(sp.expectations) >= len(msgs) {
		expectations := sp.expectations[0:len(msgs)]
		sp.expectations = sp.expectations[len(msgs):]

		for i, expectation := range expectations {
			if expectation.CheckFunction != nil {
				val, err := msgs[i].Value.Encode()
				if err != nil {
					sp.t.Errorf("Input message encoding failed: %s", err.Error())
					return err
				}
				errCheck := expectation.CheckFunction(val)
				if errCheck != nil {
					sp.t.Errorf("Check function returned an error: %s", errCheck.Error())
					return errCheck
				}
			}
			if expectation.Result != errProduceSuccess {
				return expectation.Result
			}
		}
		return nil
	}
	sp.t.Errorf("Insufficient expectations set on this mock producer to handle the input messages.")
	return errOutOfExpectations
}

// Close corresponds with the Close method of sarama's SyncProducer implementation.
// By closing a mock syncproducer, you also tell it that no more SendMessage calls will follow,
// so it will write an error to the test state if there's any remaining expectations.
func (sp *SyncProducer) Close() error {
	sp.l.Lock()
	defer sp.l.Unlock()

	if len(sp.expectations) > 0 {
		sp.t.Errorf("Expected to exhaust all expectations, but %d are left.", len(sp.expectations))
	}

	return nil
}

////////////////////////////////////////////////
// Setting expectations
////////////////////////////////////////////////

// ExpectSendMessageWithCheckerFunctionAndSucceed sets an expectation on the mock producer that SendMessage
// will be called. The mock producer will first call the given function to check the message value.
// It will cascade the error of the function, if any, or handle the message as if it produced
// successfully, i.e. by returning a valid partition, and offset, and a nil error.
func (sp *SyncProducer) ExpectSendMessageWithCheckerFunctionAndSucceed(cf ValueChecker) {
	sp.l.Lock()
	defer sp.l.Unlock()
	sp.expectations = append(sp.expectations, &producerExpectation{Result: errProduceSuccess, CheckFunction: cf})
}

// ExpectSendMessageWithCheckerFunctionAndFail sets an expectation on the mock producer that SendMessage will be
// called. The mock producer will first call the given function to check the message value.
// It will cascade the error of the function, if any, or handle the message as if it failed
// to produce successfully, i.e. by returning the provided error.
func (sp *SyncProducer) ExpectSendMessageWithCheckerFunctionAndFail(cf ValueChecker, err error) {
	sp.l.Lock()
	defer sp.l.Unlock()
	sp.expectations = append(sp.expectations, &producerExpectation{Result: err, CheckFunction: cf})
}

// ExpectSendMessageAndSucceed sets an expectation on the mock producer that SendMessage will be
// called. The mock producer will handle the message as if it produced successfully, i.e. by
// returning a valid partition, and offset, and a nil error.
func (sp *SyncProducer) ExpectSendMessageAndSucceed() {
	sp.ExpectSendMessageWithCheckerFunctionAndSucceed(nil)
}

// ExpectSendMessageAndFail sets an expectation on the mock producer that SendMessage will be
// called. The mock producer will handle the message as if it failed to produce
// successfully, i.e. by returning the provided error.
func (sp *SyncProducer) ExpectSendMessageAndFail(err error) {
	sp.ExpectSendMessageWithCheckerFunctionAndFail(nil, err)
}
