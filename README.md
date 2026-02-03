# how-to-build-an-agent-demo

A simple code-editing agent using Google AI Studio (Gemini API), inspired by [How to Build an Agent](https://ampcode.com/how-to-build-an-agent).

## Setup

1. Get a [Gemini API key](https://aistudio.google.com/apikey)
2. Set the environment variable:
   ```bash
   export GOOGLE_API_KEY="your-api-key"
   ```
3. Run the agent:
   ```bash
   go run main.go
   ```

## Tools

The agent has access to three tools:
- `read_file` - Read file contents
- `list_files` - List files in the current directory
- `edit_file` - Create or edit files via string replacement
