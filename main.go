package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/genai"
)

type Agent struct {
	client       *genai.Client
	conversation []*genai.Content
	tools        []*genai.Tool
}

func NewAgent(client *genai.Client) *Agent {
	return &Agent{
		client:       client,
		conversation: []*genai.Content{},
		tools:        tools,
	}
}

var tools = []*genai.Tool{
	{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "read_file",
				Description: "Read the contents of a given relative file path. Use this when you want to see what's inside a file. Do not use this for binary files.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"path": {
							Type:        genai.TypeString,
							Description: "The relative path to the file to read",
						},
					},
					Required: []string{"path"},
				},
			},
			{
				Name:        "list_files",
				Description: "List all files and directories in the current working directory. Returns a list of file and directory names. Directories end with a trailing slash.",
				Parameters: &genai.Schema{
					Type:       genai.TypeObject,
					Properties: map[string]*genai.Schema{},
				},
			},
			{
				Name:        "edit_file",
				Description: "Edit a file by replacing old text with new text. If the file doesn't exist, it will be created with the new text.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"path": {
							Type:        genai.TypeString,
							Description: "The relative path to the file to edit",
						},
						"old_str": {
							Type:        genai.TypeString,
							Description: "The text to replace (must match exactly). Empty string creates a new file.",
						},
						"new_str": {
							Type:        genai.TypeString,
							Description: "The new text to replace with",
						},
					},
					Required: []string{"path", "old_str", "new_str"},
				},
			},
		},
	},
}

func ReadFile(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}
	return string(content)
}

func ListFiles() []string {
	entries, err := os.ReadDir(".")
	if err != nil {
		return []string{fmt.Sprintf("Error listing files: %v", err)}
	}

	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		files = append(files, name)
	}
	return files
}

func EditFile(path, oldStr, newStr string) string {
	if oldStr == "" {
		return createNewFile(path, newStr)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}

	oldContent := string(content)
	if !strings.Contains(oldContent, oldStr) {
		return fmt.Sprintf("Error: old_str not found in file")
	}

	newContent := strings.Replace(oldContent, oldStr, newStr, 1)
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return fmt.Sprintf("Error writing file: %v", err)
	}

	return "OK"
}

func createNewFile(path, content string) string {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Sprintf("Error creating file: %v", err)
	}
	return "OK"
}

func (a *Agent) executeTool(name string, args map[string]any) string {
	switch name {
	case "read_file":
		path, _ := args["path"].(string)
		return ReadFile(path)
	case "list_files":
		files := ListFiles()
		result, _ := json.Marshal(files)
		return string(result)
	case "edit_file":
		path, _ := args["path"].(string)
		oldStr, _ := args["old_str"].(string)
		newStr, _ := args["new_str"].(string)
		return EditFile(path, oldStr, newStr)
	default:
		return fmt.Sprintf("Unknown tool: %s", name)
	}
}

func (a *Agent) runInference(ctx context.Context) (*genai.GenerateContentResponse, error) {
	config := &genai.GenerateContentConfig{
		Tools: a.tools,
	}

	return a.client.Models.GenerateContent(ctx, "gemini-2.5-flash", a.conversation, config)
}

func (a *Agent) Run(ctx context.Context, userMessage string) (string, error) {
	a.conversation = append(a.conversation, genai.NewContentFromText(userMessage, genai.RoleUser))

	for {
		response, err := a.runInference(ctx)
		if err != nil {
			return "", err
		}

		if len(response.Candidates) == 0 || response.Candidates[0].Content == nil {
			return "", fmt.Errorf("no response from model")
		}

		assistantContent := response.Candidates[0].Content
		a.conversation = append(a.conversation, assistantContent)

		functionCalls := response.FunctionCalls()
		if len(functionCalls) == 0 {
			return response.Text(), nil
		}

		var functionResponseParts []*genai.Part
		for _, fc := range functionCalls {
			fmt.Printf("tool : %s(%s)\n", fc.Name, argsToString(fc.Args))
			result := a.executeTool(fc.Name, fc.Args)
			part := genai.NewPartFromFunctionResponse(fc.Name, map[string]any{"result": result})
			functionResponseParts = append(functionResponseParts, part)
		}

		a.conversation = append(a.conversation, &genai.Content{
			Role:  genai.RoleUser,
			Parts: functionResponseParts,
		})
	}
}

func argsToString(args map[string]any) string {
	bytes, _ := json.Marshal(args)
	return string(bytes)
}

func main() {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		log.Fatal("GOOGLE_API_KEY environment variable is not set")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	agent := NewAgent(client)

	fmt.Println("Chat with Gemini (use 'ctrl-c' to quit)")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("You : ")
		if !scanner.Scan() {
			break
		}
		userInput := scanner.Text()
		if userInput == "" {
			continue
		}

		response, err := agent.Run(ctx, userInput)
		if err != nil {
			log.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("Gemini : %s\n\n", response)
	}
}
