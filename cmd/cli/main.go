package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/raphael-foliveira/tca/pkg/agent"
	"github.com/raphael-foliveira/tca/pkg/repository"
	"github.com/raphael-foliveira/tca/pkg/service"
	"github.com/raphael-foliveira/tca/pkg/tools"
)

func main() {
	openaiApiKey := getEnv("OPENAI_API_KEY", "")
	modelName := getEnv("OPENAI_MODEL", "gpt-4o")

	openaiClient := openai.NewClient(option.WithAPIKey(openaiApiKey))
	chatCompletionService := openaiClient.Chat.Completions
	ctx := context.Background()

	systemPrompt := "You are a helpful assistant that can answer questions and help with tasks by adding as much context as possible."

	readFileTool := tools.ReadFileTool()
	getFileTreeTool := tools.GetFileTreeTool()

	// Create the chat completion client abstraction
	chatCompletionClient := agent.NewOpenAIChatCompletionClient(
		&chatCompletionService,
		openai.ChatModel(modelName),
	)

	summarizerInstance := agent.NewSummarizer(
		agent.SummarizerConfig{
			ChatClient: chatCompletionClient,
		},
	)

	chatAgent := agent.NewAgent(
		agent.AgentConfig{
			ChatClient:   chatCompletionClient,
			SystemPrompt: systemPrompt,
			Tools: []*tools.Tool{
				readFileTool,
				getFileTreeTool,
			},
		},
	)

	db, err := pgxpool.New(ctx, "postgres://postgres:postgres@localhost:5432/postgres")
	if err != nil {
		log.Fatal(err)
	}
	checkpointsRepository := repository.NewPGXCheckpointsRepository(db)
	sessionsRepository := repository.NewPGXSessionsRepository(db)
	llmService := service.NewLLMService(
		chatAgent,
		checkpointsRepository,
		summarizerInstance,
	)

	chat := NewCommandLineChat(
		llmService,
		checkpointsRepository,
		sessionsRepository,
	)
	if err := chat.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

type CommandLineChat struct {
	llmService            service.LLMService
	checkpointsRepository repository.CheckpointsRepository
	sessionsRepository    repository.SessionsRepository
}

func NewCommandLineChat(
	llmService service.LLMService,
	checkpointsRepository repository.CheckpointsRepository,
	sessionsRepository repository.SessionsRepository,
) *CommandLineChat {
	return &CommandLineChat{
		llmService:            llmService,
		checkpointsRepository: checkpointsRepository,
		sessionsRepository:    sessionsRepository,
	}
}

func (c *CommandLineChat) Run(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)

	sessionId := "19be545c-7eb5-42a2-a8e2-dfddc10f4765"

	for {
		fmt.Print("\nYou: ")
		userMessage, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read user message: %w", err)
		}

		userMessage = strings.TrimSpace(userMessage)
		if isExitMessage(userMessage) {
			return nil
		}

		fmt.Print("Assistant: ")
		if err := c.llmService.InvokeStream(
			ctx,
			sessionId,
			userMessage,
			func(content string) error {
				_, err := fmt.Print(content)
				return err
			},
		); err != nil {
			fmt.Println()
			return fmt.Errorf("failed to invoke LLM service: %w", err)
		}

		fmt.Println()
	}
}

func isExitMessage(msg string) bool {
	return slices.Contains(
		[]string{"exit", "quit"},
		strings.ToLower(msg),
	)
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
