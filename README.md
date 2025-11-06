# TCA

A Go project for working with LLM tool calls and function calling capabilities. Provides an extensible framework for building conversational AI agents with tool support, conversation history persistence, and streaming responses.

## Features

- Provider-specific agent architecture (OpenAI) with tool calling support
- Streaming and non-streaming chat completions
- Conversation history management with PostgreSQL
- Context summarization for long conversations
- Built-in file system tools (read files, get file tree)
- Extensible tool system with type-safe parameters and JSON schema generation
- Built-in tool execution with support for extra tools
- Direct OpenAI SDK integration (v2)

## Prerequisites

- Go 1.23.0 or higher
- Docker and Docker Compose
- goose (database migration tool)
- OpenAI API key
- tree command (for file tree tool)

## Installation

1. Clone the repository:

```bash
git clone https://github.com/raphael-foliveira/tca.git
cd tca
```

2. Install dependencies:

```bash
go mod download
```

3. Install goose for database migrations:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

## Configuration

1. Copy the example environment file:

```bash
cp .envrc.example .envrc
```

2. Edit `.envrc` and set your OpenAI API key:

```bash
export OPENAI_API_KEY="your-actual-api-key-here"
```

3. (Optional) If using direnv, allow the environment file:

```bash
direnv allow
```

Otherwise, source the environment file manually:

```bash
source .envrc
```

## Database Setup

1. Start the PostgreSQL database (with pgvector extension):

```bash
make run-db
```

2. Run database migrations:

```bash
make migrate
```

## Development

### Available Make Commands

- `make test` - Run all tests
- `make test-coverage` - Run tests with coverage report and generate HTML report
- `make lint` - Run code linting with go vet
- `make format` - Format code with go fmt
- `make build` - Build the CLI binary to build/cli-agent
- `make run-cli` - Build and run the CLI application
- `make run-db` - Start the PostgreSQL database container
- `make migrate` - Run database migrations

### Running Tests

```bash
make test
```

For test coverage:

```bash
make test-coverage
```

### Building the Project

```bash
make build
```

This creates an executable binary at `build/cli-agent`.

### Running the CLI

After building:

```bash
./build/cli-agent
```

Or use the shorthand:

```bash
make run-cli
```

The CLI provides an interactive chat interface. Type your messages and the assistant will respond using available tools. Type `exit` or `quit` to end the session.

## Database

The project uses PostgreSQL with the pgvector extension, running in Docker. The database runs on `localhost:5432` with default credentials (see `.envrc.example`).

Database schema includes:

- `sessions` - Conversation sessions
- `checkpoints` - Message history and conversation state for each session

## Usage

### Creating Custom Tools

You can create custom tools using the type-safe tool builder:

```go
import (
    "context"
    "github.com/raphael-foliveira/tca/pkg/tools"
)

type MyToolParams struct {
    Query string `json:"query" jsonschema:"description=The search query"`
}

myTool := tools.NewTool(
    "my_tool_name",
    "Description of what the tool does",
    func(ctx context.Context, params MyToolParams) (string, error) {
        // Tool implementation
        return "result", nil
    },
)
```

### Configuring an OpenAI Agent

```go
import (
    "github.com/openai/openai-go/v2"
    "github.com/openai/openai-go/v2/option"
    "github.com/raphael-foliveira/tca/pkg/agent"
    "github.com/raphael-foliveira/tca/pkg/tools"
)

// Initialize OpenAI client
openaiClient := openai.NewClient(option.WithAPIKey("your-api-key"))
chatCompletionService := openaiClient.Chat.Completions

// Create the chat completion client abstraction
chatCompletionClient := agent.NewOpenAIChatCompletionClient(
    &chatCompletionService,
    openai.ChatModelGPT4o,
)

// Create agent with tools
chatAgent := agent.NewAgent(
    agent.AgentConfig{
        ChatClient:   chatCompletionClient,
        SystemPrompt: "You are a helpful assistant.",
        Tools:        []*tools.Tool{tool1, tool2},
    },
)
```

## Environment Variables

See `.envrc.example` for all available environment variables:

- `OPENAI_API_KEY` - Your OpenAI API key (required)
- OPENAI_MODEL - OpenAI model to use (default: gpt-4o)
- `DATABASE_URL` - PostgreSQL connection string
- `GOOSE_DRIVER` - Database driver for goose (postgres)
- `GOOSE_DBSTRING` - Database connection string for goose
- `GOOSE_MIGRATION_DIR` - Migration files directory

## License

This project is open source.
