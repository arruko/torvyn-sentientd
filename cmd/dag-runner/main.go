package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

var sleepFn = time.Sleep

// TaskInput represents the input to a DAG task
type TaskInput struct {
	TaskID     string                 `json:"task_id"`
	TaskName   string                 `json:"task_name"`
	Tool       string                 `json:"tool"`
	Inputs     map[string]interface{} `json:"inputs"`
	IncidentID string                 `json:"incident_id"`
	PlanID     string                 `json:"plan_id"`
}

func main() {
	// Parse command-line flags
	taskID := flag.String("task-id", "", "Task ID")
	taskName := flag.String("task-name", "", "Task name")
	tool := flag.String("tool", "", "Tool to execute")
	inputsJSON := flag.String("inputs", "{}", "Task inputs as JSON")
	flag.Parse()

	// Get environment variables
	incidentID := os.Getenv("INCIDENT_ID")
	planID := os.Getenv("PLAN_ID")

	// Parse inputs
	var inputs map[string]interface{}
	if err := json.Unmarshal([]byte(*inputsJSON), &inputs); err != nil {
		log.Fatalf("Failed to parse inputs JSON: %v", err)
	}

	// Build task input
	task := TaskInput{
		TaskID:     *taskID,
		TaskName:   *taskName,
		Tool:       *tool,
		Inputs:     inputs,
		IncidentID: incidentID,
		PlanID:     planID,
	}

	log.Printf("=== DAG Task Execution Started ===")
	log.Printf("Incident ID: %s", task.IncidentID)
	log.Printf("Plan ID: %s", task.PlanID)
	log.Printf("Task ID: %s", task.TaskID)
	log.Printf("Task Name: %s", task.TaskName)
	log.Printf("Tool: %s", task.Tool)
	log.Printf("==================================")

	// Execute the task
	if err := executeTask(task); err != nil {
		log.Printf("❌ Task execution failed: %v", err)
		log.Fatalf("FATAL: incident=%s plan=%s task=%s tool=%s error=%v",
			task.IncidentID, task.PlanID, task.TaskID, task.Tool, err)
	}

	log.Printf("✅ Task completed successfully: incident=%s plan=%s task=%s tool=%s",
		task.IncidentID, task.PlanID, task.TaskID, task.Tool)
}

// executeTask executes a single DAG task
func executeTask(task TaskInput) error {
	switch task.Tool {
	case "kubectl":
		return executeKubectl(task)
	case "helm":
		return executeHelm(task)
	case "http":
		return executeHTTP(task)
	case "script":
		return executeScript(task)
	case "wait":
		return executeWait(task)
	default:
		return fmt.Errorf("unknown tool: %s", task.Tool)
	}
}

// executeKubectl executes kubectl commands
func executeKubectl(task TaskInput) error {
	log.Printf("[kubectl] Executing task: %s", task.TaskName)

	action, ok := task.Inputs["action"].(string)
	if !ok {
		return fmt.Errorf("kubectl task missing 'action' input")
	}

	resource, ok := task.Inputs["resource"].(string)
	if !ok {
		return fmt.Errorf("kubectl task missing 'resource' input")
	}

	log.Printf("[kubectl] Action: %s, Resource: %s", action, resource)
	log.Printf("[kubectl] Would execute: kubectl %s %s", action, resource)

	sleepFn(2 * time.Second)
	return nil
}

// executeHelm executes Helm commands
func executeHelm(task TaskInput) error {
	log.Printf("[helm] Executing task: %s", task.TaskName)

	action, ok := task.Inputs["action"].(string)
	if !ok {
		return fmt.Errorf("helm task missing 'action' input")
	}

	release, ok := task.Inputs["release"].(string)
	if !ok {
		return fmt.Errorf("helm task missing 'release' input")
	}

	log.Printf("[helm] Action: %s, Release: %s", action, release)
	log.Printf("[helm] Would execute: helm %s %s", action, release)

	sleepFn(2 * time.Second)
	return nil
}

// executeHTTP executes HTTP requests
func executeHTTP(task TaskInput) error {
	log.Printf("[http] Executing task: %s", task.TaskName)

	method, ok := task.Inputs["method"].(string)
	if !ok {
		method = "GET"
	}

	url, ok := task.Inputs["url"].(string)
	if !ok {
		return fmt.Errorf("http task missing 'url' input")
	}

	log.Printf("[http] Method: %s, URL: %s", method, url)
	log.Printf("[http] Would execute: %s %s", method, url)

	sleepFn(1 * time.Second)
	return nil
}

// executeScript executes arbitrary scripts (disabled unless ALLOW_SCRIPT=true)
func executeScript(task TaskInput) error {
	log.Printf("[script] Executing task: %s", task.TaskName)

	if os.Getenv("ALLOW_SCRIPT") != "true" {
		return fmt.Errorf("script tool disabled by runtime policy (set ALLOW_SCRIPT=true to enable)")
	}

	script, ok := task.Inputs["script"].(string)
	if !ok {
		return fmt.Errorf("script task missing 'script' input")
	}

	log.Printf("[script] Script length: %d bytes", len(script))
	log.Printf("[script] Would execute script: %s", script)

	sleepFn(2 * time.Second)
	return nil
}

// executeWait waits for a specified duration
func executeWait(task TaskInput) error {
	log.Printf("[wait] Executing task: %s", task.TaskName)

	durationStr, ok := task.Inputs["duration"].(string)
	if !ok {
		return fmt.Errorf("wait task missing 'duration' input")
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	log.Printf("[wait] Waiting for %s", duration)
	sleepFn(duration)

	return nil
}
