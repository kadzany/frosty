package internal

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/kadzany/frosty/workflow"
	"github.com/stretchr/testify/assert"
)

func TestWorkflowHandler_CreateNode(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	handler := WorkflowHandler{DB: db}
	node := workflow.Node{
		Title:       "Test Node",
		Type:        "Task",
		Description: "Test Description",
	}
	nodeJSON, _ := json.Marshal(node)

	mock.ExpectQuery("INSERT INTO nodes").
		WithArgs("Test Node", "Task", "Test Description").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	req, _ := http.NewRequest("POST", "/workflow/node", bytes.NewBuffer(nodeJSON))
	resw := httptest.NewRecorder()

	handler.CreateNode(resw, req)

	assert.Equal(t, http.StatusCreated, resw.Code)
}

func TestWorkflowHandler_GetNode(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	handler := WorkflowHandler{DB: db}
	nodeID := uuid.New()

	mock.ExpectQuery("SELECT id::uuid, title, type, description, created_at, updated_at, deleted_at FROM nodes WHERE id = ?").
		WithArgs(nodeID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "type", "description", "created_at", "updated_at", "deleted_at"}).
			AddRow(nodeID, "Test Node", "Task", "Test Description", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), nil))

	req, _ := http.NewRequest("GET", "/workflow/node/"+nodeID.String(), nil)
	resw := httptest.NewRecorder()
	req = mux.SetURLVars(req, map[string]string{"id": nodeID.String()})

	handler.GetNode(resw, req)

	assert.Equal(t, http.StatusOK, resw.Code)
}

func TestWorkflowHandler_AddRelationship(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	handler := WorkflowHandler{DB: db}
	relationship := workflow.NodeClosure{
		Ancestor:   uuid.New(),
		Descendant: uuid.New(),
	}
	relationshipJSON, _ := json.Marshal(relationship)

	mock.ExpectExec("INSERT INTO node_closure").WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest("POST", "/workflow/node/relationship", bytes.NewBuffer(relationshipJSON))
	resw := httptest.NewRecorder()

	handler.AddRelationship(resw, req)

	assert.Equal(t, http.StatusCreated, resw.Code)
}

func TestWorkflowHandler_GetDescendants(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	handler := WorkflowHandler{DB: db}
	nodeID := uuid.New()

	mock.ExpectQuery("SELECT n.id, n.title, n.type, n.description, n.created_at, n.updated_at, n.deleted_at FROM nodes n JOIN node_closure nc ON nc.descendant = n.id WHERE nc.ancestor = ?").
		WithArgs(nodeID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "type", "description", "created_at", "updated_at", "deleted_at"}).
			AddRow(uuid.New(), "Child Node", "Task", "Child Description", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), nil))

	req, _ := http.NewRequest("GET", "/workflow/node/"+nodeID.String()+"/descendants", nil)
	resw := httptest.NewRecorder()
	req = mux.SetURLVars(req, map[string]string{"id": nodeID.String()})

	handler.GetDescendants(resw, req)

	assert.Equal(t, http.StatusOK, resw.Code)
}

func TestWorkflowHandler_ExecuteNode_NodeStart(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	handler := WorkflowHandler{DB: db}
	nodeID := uuid.New()

	mock.ExpectQuery(`SELECT COUNT\(1\) FROM node_closure WHERE ancestor = descendant AND ancestor = \$1`).
		WithArgs(nodeID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`SELECT id::uuid, title, type, description, created_at, updated_at, deleted_at FROM nodes`).
		WithArgs(nodeID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "type", "description", "created_at", "updated_at", "deleted_at"}).
			AddRow(nodeID, "Node Title", "Start", "Description", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), nil))

	req, _ := http.NewRequest("POST", "/workflow/node/"+nodeID.String()+"/execute", nil)
	resw := httptest.NewRecorder()
	req = mux.SetURLVars(req, map[string]string{"id": nodeID.String()})

	handler.ExecuteNode(resw, req)

	log.Println(nodeID.String())
	log.Println(resw.Body.String())

	assert.Equal(t, http.StatusOK, resw.Code)
}

func TestWorkflowHandler_RollbackNode(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	handler := WorkflowHandler{DB: db}
	nodeID := uuid.New()

	mock.ExpectQuery("SELECT n.id, n.title, n.type, n.description, n.created_at, n.updated_at, n.deleted_at").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "type", "description", "created_at", "updated_at", "deleted_at"}).
			AddRow(nodeID, "Node Title", "Task", "Description", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), nil))

	mock.ExpectExec(`INSERT INTO workflow_logs \(node_id, status, message\) VALUES \(\$1\:\:uuid, \$2, \$3\)`).
		WithArgs(nodeID, "rollback", "Node rolled back successfully").
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest("POST", "/workflow/node/"+nodeID.String()+"/rollback", nil)
	resw := httptest.NewRecorder()
	req = mux.SetURLVars(req, map[string]string{"id": nodeID.String()})

	handler.RollbackNode(resw, req)

	assert.Equal(t, http.StatusOK, resw.Code)
}
