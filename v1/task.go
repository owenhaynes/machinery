package machinery

import (
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"

	"github.com/RichardKnop/machinery/v1/backends"
	"github.com/RichardKnop/machinery/v1/logger"
	"github.com/RichardKnop/machinery/v1/signatures"
	"github.com/RichardKnop/machinery/v1/utils"
)

var (
	// ErrTaskPanicked ...
	ErrTaskPanicked = errors.New("Invoking task caused a panic")
	// ErrTaskReturnedNoValues ...
	ErrTaskReturnedNoValues = errors.New("Task returned no values. At least an error return value is required")
	// ErrLastReturnValueMustBeError ..
	ErrLastReturnValueMustBeError = errors.New("Last return value of a task must be error")
)

// Task wraps a signature and methods used to reflect task arguments and
// return values after invoking the task
type Task struct {
	TaskFunc reflect.Value
	Args     []reflect.Value
}

// NewTask tries to use reflection to convert the function and arguments
// into a reflect.Value and prepare it for invocation
func NewTask(taskFunc interface{}, args []signatures.TaskArg) (*Task, error) {
	task := &Task{
		TaskFunc: reflect.ValueOf(taskFunc),
	}

	if err := task.ReflectArgs(args); err != nil {
		return nil, fmt.Errorf("Reflect task args: %v", err)
	}

	return task, nil
}

// Call attempts to call the task with the supplied arguments.
//
// `err` is set in the return value in two cases:
// 1. The reflected function invocation panics (e.g. due to a mismatched
//    argument list).
// 2. The task func itself returns a non-nil error.
func (t *Task) Call() (taskResults []*backends.TaskResult, err error) {
	defer func() {
		// Recover from panic and set err.
		if e := recover(); e != nil {
			switch e := e.(type) {
			default:
				err = ErrTaskPanicked
			case error:
				err = e
			case string:
				err = errors.New(e)
			}
			// Print stack trace
			logger.Get().Printf("%s", debug.Stack())
		}
	}()

	// Invoke the task
	results := t.TaskFunc.Call(t.Args)

	// Task must return at least a single error argument
	if len(results) == 0 {
		return nil, ErrTaskReturnedNoValues
	}

	lastResult := results[len(results)-1]

	// If an error was returned by the task func, propagate it
	// to the caller via err.
	if !lastResult.IsNil() {
		errorInterface := reflect.TypeOf((*error)(nil)).Elem()
		if !lastResult.Type().Implements(errorInterface) {
			return nil, ErrLastReturnValueMustBeError
		}

		return nil, lastResult.Interface().(error)
	}

	// Convert reflect values to task results
	taskResults = make([]*backends.TaskResult, len(results)-1)
	for i := 0; i < len(results)-1; i++ {
		taskResults[i] = &backends.TaskResult{
			Type:  reflect.TypeOf(results[i].Interface()).String(),
			Value: results[i].Interface(),
		}
	}

	return taskResults, err
}

// ReflectArgs converts []TaskArg to []reflect.Value
func (t *Task) ReflectArgs(args []signatures.TaskArg) error {
	argValues := make([]reflect.Value, len(args))

	for i, arg := range args {
		argValue, err := utils.ReflectValue(arg.Type, arg.Value)
		if err != nil {
			return err
		}
		argValues[i] = argValue
	}

	t.Args = argValues
	return nil
}
