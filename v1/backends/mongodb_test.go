package backends_test

import (
	"os"
	"testing"

	"github.com/RichardKnop/machinery/v1/backends"
	"github.com/RichardKnop/machinery/v1/config"
	"github.com/RichardKnop/machinery/v1/signatures"
	"github.com/stretchr/testify/assert"
)

var (
	groupUUID = "123456"
	taskUUIDs = []string{"1", "2", "3"}
)

func initTestMongodbBackend() (backends.Backend, error) {
	conf := &config.Config{
		ResultBackend:   os.Getenv("MONGODB_URL"),
		ResultsExpireIn: 30,
	}
	backend, err := backends.NewMongodbBackend(conf)
	if err != nil {
		return nil, err
	}

	backend.PurgeGroupMeta(groupUUID)
	for _, taskUUID := range taskUUIDs {
		backend.PurgeState(taskUUID)
	}

	err = backend.InitGroup(groupUUID, taskUUIDs)
	if err != nil {
		return nil, err
	}
	return backend, nil
}

func TestNewMongodbBackend(t *testing.T) {
	if os.Getenv("MONGODB_URL") == "" {
		return
	}

	backend, err := initTestMongodbBackend()
	if assert.NoError(t, err) {
		assert.NotNil(t, backend)
	}
}

func TestSetStatePending(t *testing.T) {
	if os.Getenv("MONGODB_URL") == "" {
		return
	}

	backend, err := initTestMongodbBackend()
	if err != nil {
		t.Fatal(err)
	}

	err = backend.SetStatePending(&signatures.TaskSignature{
		UUID: taskUUIDs[0],
	})
	if assert.NoError(t, err) {
		taskState, err := backend.GetState(taskUUIDs[0])
		if assert.NoError(t, err) {
			assert.Equal(t, backends.PendingState, taskState.State, "Not PendingState")
		}
	}
}

func TestSetStateReceived(t *testing.T) {
	if os.Getenv("MONGODB_URL") == "" {
		return
	}

	backend, err := initTestMongodbBackend()
	if err != nil {
		t.Fatal(err)
	}

	err = backend.SetStateReceived(&signatures.TaskSignature{
		UUID: taskUUIDs[0],
	})
	if assert.NoError(t, err) {
		taskState, err := backend.GetState(taskUUIDs[0])
		if assert.NoError(t, err) {
			assert.Equal(t, backends.ReceivedState, taskState.State, "Not ReceivedState")
		}
	}
}

func TestSetStateStarted(t *testing.T) {
	if os.Getenv("MONGODB_URL") == "" {
		return
	}

	backend, err := initTestMongodbBackend()
	if err != nil {
		t.Fatal(err)
	}

	err = backend.SetStateStarted(&signatures.TaskSignature{
		UUID: taskUUIDs[0],
	})
	if assert.NoError(t, err) {
		taskState, err := backend.GetState(taskUUIDs[0])
		if assert.NoError(t, err) {
			assert.Equal(t, backends.StartedState, taskState.State, "Not StartedState")
		}
	}
}

func TestSetStateSuccess(t *testing.T) {
	if os.Getenv("MONGODB_URL") == "" {
		return
	}

	resultType := "int64"
	resultValue := int64(88)

	backend, err := initTestMongodbBackend()
	if err != nil {
		t.Fatal(err)
	}

	signature := &signatures.TaskSignature{
		UUID: taskUUIDs[0],
	}
	taskResults := []*backends.TaskResult{
		&backends.TaskResult{
			Type:  resultType,
			Value: resultValue,
		},
	}
	err = backend.SetStateSuccess(signature, taskResults)
	assert.NoError(t, err)

	taskState, err := backend.GetState(taskUUIDs[0])
	assert.NoError(t, err)
	assert.Equal(t, backends.SuccessState, taskState.State, "Not SuccessState")
	assert.Equal(t, resultType, taskState.Results[0].Type, "Wrong result type")
	assert.Equal(t, float64(resultValue), taskState.Results[0].Value.(float64), "Wrong result value")
}

func TestSetStateFailure(t *testing.T) {
	if os.Getenv("MONGODB_URL") == "" {
		return
	}

	failStrig := "Fail is ok"

	backend, err := initTestMongodbBackend()
	if err != nil {
		t.Fatal(err)
	}

	signature := &signatures.TaskSignature{
		UUID: taskUUIDs[0],
	}
	err = backend.SetStateFailure(signature, failStrig)
	assert.NoError(t, err)

	taskState, err := backend.GetState(taskUUIDs[0])
	assert.NoError(t, err)
	assert.Equal(t, backends.FailureState, taskState.State, "Not SuccessState")
	assert.Equal(t, failStrig, taskState.Error, "Wrong fail error")
}

func TestGroupCompleted(t *testing.T) {
	if os.Getenv("MONGODB_URL") == "" {
		return
	}

	backend, err := initTestMongodbBackend()
	if err != nil {
		t.Fatal(err)
	}
	taskResultsState := make(map[string]string)

	isCompleted, err := backend.GroupCompleted(groupUUID, len(taskUUIDs))
	if assert.NoError(t, err) {
		assert.False(t, isCompleted, "Actualy group is not completed")
	}

	signature := &signatures.TaskSignature{
		UUID: taskUUIDs[0],
	}
	err = backend.SetStateFailure(signature, "Fail is ok")
	assert.NoError(t, err)
	taskResultsState[taskUUIDs[0]] = backends.FailureState

	signature = &signatures.TaskSignature{
		UUID: taskUUIDs[1],
	}
	taskResults := []*backends.TaskResult{
		&backends.TaskResult{
			Type:  "string",
			Value: "Result ok",
		},
	}
	err = backend.SetStateSuccess(signature, taskResults)
	assert.NoError(t, err)
	taskResultsState[taskUUIDs[1]] = backends.SuccessState

	signature = &signatures.TaskSignature{
		UUID: taskUUIDs[2],
	}
	err = backend.SetStateSuccess(signature, taskResults)
	assert.NoError(t, err)
	taskResultsState[taskUUIDs[2]] = backends.SuccessState

	isCompleted, err = backend.GroupCompleted(groupUUID, len(taskUUIDs))
	if assert.NoError(t, err) {
		assert.True(t, isCompleted, "Actualy group is completed")
	}

	groupTasksStates, err := backend.GroupTaskStates(groupUUID, len(taskUUIDs))
	assert.NoError(t, err)

	assert.Equal(t, len(groupTasksStates), len(taskUUIDs), "Wrong len tasksStates")
	for i := range groupTasksStates {
		assert.Equal(
			t,
			taskResultsState[groupTasksStates[i].TaskUUID],
			groupTasksStates[i].State,
			"Wrong state on", groupTasksStates[i].TaskUUID,
		)
	}
}

func TestMongodbDropIndexes(t *testing.T) {
	mongoDBURL := os.Getenv("MONGODB_URL")
	if mongoDBURL == "" {
		return
	}

	conf := &config.Config{
		ResultBackend:   mongoDBURL,
		ResultsExpireIn: 5,
	}

	_, err := backends.NewMongodbBackend(conf)
	assert.NoError(t, err)

	conf.ResultsExpireIn = 7

	_, err = backends.NewMongodbBackend(conf)
	assert.NoError(t, err)
}
