package workflow

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ============================================ Node ============================================

func CreateNode(db *sql.DB, title, nodeType string, description string) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(`
		INSERT INTO nodes (title, type, description, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id
	`, title, nodeType, description).Scan(&id)

	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func GetNode(db *sql.DB, nodeID uuid.UUID) (Node, error) {
	node := Node{}
	err := db.QueryRow(`
		SELECT id::uuid, title, type, description, created_at, updated_at, deleted_at
		FROM nodes
		WHERE id = $1
	`, nodeID).Scan(&node.ID, &node.Title, &node.Type, &node.Description, &node.CreatedAt, &node.UpdatedAt, &node.DeletedAt)
	return node, err
}

func AddRelationship(db *sql.DB, ancestor, descendant uuid.UUID) error {
	_, err := db.Exec(`
		INSERT INTO node_closure (ancestor, descendant, depth)
		SELECT ancestor, $1::uuid, depth + 1
		FROM node_closure
		WHERE descendant = $2::uuid
		UNION ALL
		SELECT $3::uuid, $4::uuid, 0
	`, descendant, ancestor, ancestor, descendant)

	return err
}

func GetDescendants(db *sql.DB, ancestor uuid.UUID) ([]Node, error) {
	rows, err := db.Query(`
		SELECT n.id, n.title, n.type, n.description, n.created_at, n.updated_at, n.deleted_at
		FROM nodes n
		JOIN node_closure nc ON nc.descendant = n.id
		WHERE nc.ancestor = $1
	`, ancestor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var descendants []Node
	for rows.Next() {
		node := Node{}
		err := rows.Scan(&node.ID, &node.Title, &node.Type, &node.Description, &node.CreatedAt, &node.UpdatedAt, &node.DeletedAt)
		if err != nil {
			return nil, err
		}
		descendants = append(descendants, node)
	}
	return descendants, nil
}

func GetImmediateAncestor(db *sql.DB, nodeID uuid.UUID) (Node, error) {
	row := db.QueryRow(`
		SELECT n.id, n.title, n.type, n.description, n.created_at, n.updated_at, n.deleted_at
		FROM node_closure nc
		JOIN nodes n ON nc.ancestor = n.id
		WHERE nc.descendant = $1::uuid AND nc.depth = 1
		LIMIT 1
	`, nodeID)

	node := Node{}
	err := row.Scan(&node.ID, &node.Title, &node.Type, &node.Description, &node.CreatedAt, &node.UpdatedAt, &node.DeletedAt)
	if err != nil {
		return Node{}, err
	}
	return node, nil
}

func AllParentsCompleted(db *sql.DB, nodeID uuid.UUID) bool {
	var count int
	err := db.QueryRow(`
        SELECT COUNT(*)
        FROM node_closure nc
        JOIN nodes n ON nc.ancestor = n.id
        WHERE nc.descendant = $1::uuid AND n.type != 'End'
    `, nodeID).Scan(&count)

	if err != nil {
		return false
	}
	return count == 0
}

// ============================================ Workflow ============================================

func ValidateWorkflow(db *sql.DB, startNode uuid.UUID) error {
	rows := db.QueryRow("SELECT COUNT(1) FROM node_closure WHERE ancestor = descendant AND ancestor = $1::uuid", startNode)

	var count int
	err := rows.Scan(&count)
	if err != nil {
		return err
	}
	if count > 1 {
		return fmt.Errorf("cyclic dependency detected")
	}
	return nil
}

func GetExecutedNodes(db *sql.DB, currentNode uuid.UUID) ([]Node, error) {
	rows, err := db.Query(`
		SELECT n.id, n.title, n.type, n.description, n.created_at, n.updated_at, n.deleted_at
		FROM workflow_logs wl
		JOIN nodes n ON wl.node_id = n.id
		WHERE wl.status = 'success' AND wl.executed_at <= (
			SELECT executed_at FROM workflow_logs WHERE node_id = $1::uuid
		)
		ORDER BY wl.executed_at DESC
	`, currentNode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		node := Node{}
		err := rows.Scan(&node.ID, &node.Title, &node.Type, &node.Description, &node.CreatedAt, &node.UpdatedAt, &node.DeletedAt)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func GetWorkflowNodes(db *sql.DB, workflowID uuid.UUID) ([]Node, error) {
	// Query to fetch all nodes belonging to the specified workflow
	query := `
		SELECT
			n.id, n.title, n.type, n.description, n.created_at, n.updated_at, n.deleted_at
		FROM
			nodes n
		INNER JOIN
			workflow_starting_nodes wn ON n.id = wn.starting_node_id
		WHERE
			wn.workflow_id = $1 AND n.deleted_at IS NULL
		ORDER BY
			wn.created_at ASC;
	`

	// Execute the query
	rows, err := db.Query(query, workflowID)
	if err != nil {
		return nil, fmt.Errorf("error fetching nodes for workflow %s: %v", workflowID, err)
	}
	defer rows.Close()

	var nodes []Node

	// Parse the rows into Node structs
	for rows.Next() {
		var node Node
		err := rows.Scan(&node.ID, &node.Title, &node.Type, &node.Description, &node.CreatedAt, &node.UpdatedAt, &node.DeletedAt)
		if err != nil {
			return nil, fmt.Errorf("error scanning node row: %v", err)
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func LogWorkflowNodeExecution(db *sql.DB, workflowID, nodeID uuid.UUID, status, message string) error {
	_, err := db.Exec(`
		INSERT INTO workflow_logs (workflow_id, node_id, status, message, executed_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5)
	`, workflowID, nodeID, status, message, time.Now())
	return err
}

func GetStartingNode(db *sql.DB, workflowID uuid.UUID) (Node, error) {
	// Query to fetch the starting node of the workflow
	query := `
		SELECT
			n.id, n.title, n.type, n.description, n.created_at, n.updated_at, n.deleted_at
		FROM
			nodes n
		INNER JOIN
			workflow_starting_nodes wn ON n.id = wn.starting_node_id
		WHERE
			wn.workflow_id = $1 AND n.deleted_at IS NULL
		LIMIT 1;
	`

	// Execute the query
	row := db.QueryRow(query, workflowID)

	var node Node

	// Parse the result into a Node struct
	err := row.Scan(&node.ID, &node.Title, &node.Type, &node.Description, &node.CreatedAt, &node.UpdatedAt, &node.DeletedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Node{}, fmt.Errorf("no starting node found for workflow %s", workflowID)
		}
		return Node{}, fmt.Errorf("error fetching starting node for workflow %s: %v", workflowID, err)
	}

	return node, nil
}

func UpdateWorkflowStatus(db *sql.DB, workflowID uuid.UUID, status string) error {
	_, err := db.Exec(`
		UPDATE workflows
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`, status, workflowID)
	return err
}

// ============================================ Task ============================================

func GetNodeTasks(db *sql.DB, nodeID uuid.UUID) ([]NodeTask, error) {
	// Query to fetch tasks associated with the given node
	query := `
		SELECT
			nt.id,
			nt.node_id,
			nt.task_id,
			nt.task_order,
			nt.status,
			nt.retry_count,
			nt.http_code,
			nt.response,
			nt.error,
			nt.created_at,
			nt.updated_at,
			nt.deleted_at,
			t.id,
			t.title,
			t.type,
			t.http_method,
			t.action,
			t.params
		FROM
			node_tasks nt
		INNER JOIN
			tasks t ON nt.task_id = t.id
		WHERE
			nt.node_id = $1 AND nt.deleted_at IS NULL
		ORDER BY
			nt.task_order ASC;
	`

	// Execute the query
	rows, err := db.Query(query, nodeID)
	if err != nil {
		return nil, fmt.Errorf("error fetching tasks for node %s: %v", nodeID, err)
	}
	defer rows.Close()

	var nodeTasks []NodeTask

	// Parse the rows into NodeTask structs
	for rows.Next() {
		var nodeTask NodeTask

		err := rows.Scan(
			&nodeTask.ID, &nodeTask.NodeID, &nodeTask.TaskID,
			&nodeTask.TaskOrder, &nodeTask.Status, &nodeTask.RetryCount,
			&nodeTask.HttpCode, &nodeTask.Response, &nodeTask.Error,
			&nodeTask.CreatedAt, &nodeTask.UpdatedAt, &nodeTask.DeletedAt,
			&nodeTask.Task.ID, &nodeTask.Task.Title, &nodeTask.Task.Type,
			&nodeTask.Task.HttpMethod, &nodeTask.Task.Action, &nodeTask.Task.Params,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning node task row: %v", err)
		}

		nodeTasks = append(nodeTasks, nodeTask)
	}

	return nodeTasks, nil
}

func UpdateTaskStatusAndResponse(db *sql.DB, taskID uuid.UUID, status, response, errorMessage string, httpCode int) error {
	_, err := db.Exec(`
		UPDATE node_tasks
		SET status = $1, response = $2, error = $3, http_code = $4, updated_at = NOW()
		WHERE task_id = $5
	`, status, response, errorMessage, httpCode, taskID)
	return err
}

func UpdateTaskStatus(db *sql.DB, taskID uuid.UUID, status string) error {
	_, err := db.Exec(`
		UPDATE node_tasks
		SET status = $1, updated_at = NOW()
		WHERE task_id = $2
	`, status, taskID)
	return err
}

func CreateWorkflow(db *sql.DB, name, description string) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(`
		INSERT INTO workflows (name, description, created_at)
		VALUES ($1, $2, NOW())
		RETURNING id
	`, name, description).Scan(&id)

	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func CreateWorkflowStartingNode(db *sql.DB, workflowID, nodeID uuid.UUID) (uuid.UUID, error) {
	// Check if the nodeID exists in the nodes table
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM nodes WHERE id = $1)`, nodeID).Scan(&exists)
	if err != nil {
		return uuid.Nil, err
	}
	if !exists {
		return uuid.Nil, fmt.Errorf("node ID %s does not exist", nodeID)
	}

	var id uuid.UUID
	err = db.QueryRow(`
		INSERT INTO workflow_starting_nodes (workflow_id, starting_node_id, created_at)
		VALUES ($1, $2, NOW())
		RETURNING id
	`, workflowID, nodeID).Scan(&id)

	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}
