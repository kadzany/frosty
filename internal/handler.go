package internal

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/kadzany/frosty/workflow"

	"github.com/gorilla/mux"
)

type WorkflowHandler struct {
	DB *sql.DB
}

func (wh *WorkflowHandler) CreateNode(resw http.ResponseWriter, req *http.Request) {
	node := workflow.Node{}
	decoder := json.NewDecoder(req.Body)

	if err := decoder.Decode(&node); err != nil {
		responseError(resw, http.StatusBadRequest, "Invalid request payload")
	}
	defer req.Body.Close()

	id, err := workflow.CreateNode(wh.DB, node.Title, node.Type, node.Description)

	if err != nil {
		responseError(resw, http.StatusInternalServerError, err.Error())
		return
	}

	responseJson(resw, http.StatusCreated, id)
}

func (wh *WorkflowHandler) GetNode(resw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id, err := uuid.Parse(vars["id"])

	if err != nil {
		responseError(resw, http.StatusBadRequest, "Invalid Node Id")
	}

	node, err := workflow.GetNode(wh.DB, id)
	if err != nil {
		responseError(resw, http.StatusInternalServerError, err.Error())
		return
	}

	responseJson(resw, http.StatusOK, node)
}

func (wh *WorkflowHandler) AddRelationship(resw http.ResponseWriter, req *http.Request) {
	var relationship workflow.NodeClosure
	decoder := json.NewDecoder(req.Body)

	if err := decoder.Decode(&relationship); err != nil {
		responseError(resw, http.StatusBadRequest, "Invalid request payload")
	}
	defer req.Body.Close()

	err := workflow.AddRelationship(wh.DB, relationship.Ancestor, relationship.Descendant)
	if err != nil {
		responseError(resw, http.StatusInternalServerError, err.Error())
		return
	}
	responseJson(resw, http.StatusCreated, relationship)
}

func (wh *WorkflowHandler) ExecuteWorkflow(resw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id, err := uuid.Parse(vars["id"])

	if err != nil {
		responseError(resw, http.StatusBadRequest, "Invalid Workflow Id")
		return
	}

	err = workflow.ExecuteWorkflow(wh.DB, id)
	if err != nil {
		responseError(resw, http.StatusInternalServerError, err.Error())
		return
	}

	responseJson(resw, http.StatusOK, nil)
}

func (wh *WorkflowHandler) CreateWorkflow(resw http.ResponseWriter, req *http.Request) {
	wf := workflow.Workflow{}
	decoder := json.NewDecoder(req.Body)

	if err := decoder.Decode(&wf); err != nil {
		responseError(resw, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer req.Body.Close()

	id, err := workflow.CreateWorkflow(wh.DB, wf.Name, wf.Description)
	if err != nil {
		responseError(resw, http.StatusInternalServerError, err.Error())
		return
	}

	responseJson(resw, http.StatusCreated, id)
}

func (wh *WorkflowHandler) CreateWorkflowStartingNode(resw http.ResponseWriter, req *http.Request) {
	wn := workflow.WorkflowStartingNode{}
	decoder := json.NewDecoder(req.Body)

	if err := decoder.Decode(&wn); err != nil {
		responseError(resw, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer req.Body.Close()

	id, err := workflow.CreateWorkflowStartingNode(wh.DB, wn.WorkflowID, wn.StartingNodeID)
	if err != nil {
		responseError(resw, http.StatusInternalServerError, err.Error())
		return
	}

	responseJson(resw, http.StatusCreated, id)
}
