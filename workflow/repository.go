package workflow

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

func CreateNode(db *sql.DB, title, nodeType string, description string) (uuid.UUID, error) {
	id := uuid.New()
	_, err := db.Exec(`
		INSERT INTO nodes (id, title, type, description, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, id, title, nodeType, description, time.Now())

	if err != nil {
		return uuid.Nil, err
	}

	return id, err
}

func GetNode(db *sql.DB, nodeID uuid.UUID) (Node, error) {
	node := Node{}
	err := db.QueryRow(`
		SELECT id, title, type, description, created_at, updated_at, deleted_at
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

	log.Println(err)

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

func LogNodeExecution(db *sql.DB, nodeID uuid.UUID, status, message string) error {
	_, err := db.Exec(`
		INSERT INTO workflow_logs (id, node_id, status, message)
		VALUES ($1, $2, $3, $4)
	`, uuid.New(), nodeID, status, message)
	return err
}

func ValidateWorkflow(db *sql.DB, startNode uuid.UUID) error {
	rows := db.QueryRow("SELECT COUNT(1) FROM node_closure WHERE ancestor = descendant AND ancestor = $1", startNode)

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

func GetImmediateAncestors(db *sql.DB, nodeID uuid.UUID) ([]Node, error) {
	rows, err := db.Query(`
		SELECT n.id, n.title, n.type, n.description, n.created_at, n.updated_at, n.deleted_at
		FROM node_closure nc
		JOIN nodes n ON nc.ancestor = n.id
		WHERE nc.descendant = $1 AND nc.depth = 1
	`, nodeID)
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

func GetExecutedNodes(db *sql.DB, currentNode uuid.UUID) ([]Node, error) {
	rows, err := db.Query(`
		SELECT n.id, n.title, n.type, n.description, n.created_at, n.updated_at, n.deleted_at
		FROM workflow_logs wl
		JOIN nodes n ON wl.node_id = n.id
		WHERE wl.status = 'success' AND wl.executed_at <= (
			SELECT executed_at FROM workflow_logs WHERE node_id = $1
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

func AllParentsCompleted(db *sql.DB, nodeID uuid.UUID) bool {
	var count int
	err := db.QueryRow(`
        SELECT COUNT(*)
        FROM node_closure nc
        JOIN nodes n ON nc.ancestor = n.id
        WHERE nc.descendant = $1 AND n.type != 'End'
    `, nodeID).Scan(&count)

	if err != nil {
		return false
	}
	return count == 0
}