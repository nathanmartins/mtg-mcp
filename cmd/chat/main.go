package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle  = focusedStyle.Copy()
	noStyle      = lipgloss.NewStyle()

	userMsgStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	assistantMsgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
	systemMsgStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	errorMsgStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)

type (
	errMsg error
)

type model struct {
	viewport    viewport.Model
	messages    []string
	textarea    textarea.Model
	senderStyle lipgloss.Style
	err         error
	llmClient   LLMClient
	mcpClient   *MCPClient
}

type LLMClient interface {
	SendMessage(ctx context.Context, message string, tools []Tool) (*LLMResponse, error)
}

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
}

type LLMResponse struct {
	Content   string
	ToolCalls []ToolCall
}

type ToolCall struct {
	ID        string
	ToolName  string
	Arguments map[string]interface{}
}

func initialModel(llmClient LLMClient, mcpClient *MCPClient) model {
	ta := textarea.New()
	ta.Placeholder = "Ask about Magic: The Gathering cards..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 2000

	ta.SetWidth(80)
	ta.SetHeight(3)

	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()

	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to MTG Commander Assistant!\nAsk me anything about Magic: The Gathering cards.\n\n")

	ta.KeyMap.InsertNewline.SetEnabled(false)

	return model{
		textarea:    ta,
		messages:    []string{},
		viewport:    vp,
		senderStyle: userMsgStyle,
		llmClient:   llmClient,
		mcpClient:   mcpClient,
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			userMsg := m.textarea.Value()
			if userMsg == "" {
				return m, nil
			}

			// Add user message to viewport
			m.messages = append(m.messages, userMsgStyle.Render("You: ")+userMsg)
			m.viewport.SetContent(strings.Join(m.messages, "\n\n") + "\n\n")
			m.viewport.GotoBottom()
			m.textarea.Reset()

			// Send to LLM
			return m, m.sendToLLM(userMsg)
		}

	case responseMsg:
		response := string(msg)
		m.messages = append(m.messages, assistantMsgStyle.Render("Assistant: ")+response)
		m.viewport.SetContent(strings.Join(m.messages, "\n\n") + "\n\n")
		m.viewport.GotoBottom()

	case errMsg:
		m.err = msg
		errText := fmt.Sprintf("Error: %v", msg)
		m.messages = append(m.messages, errorMsgStyle.Render(errText))
		m.viewport.SetContent(strings.Join(m.messages, "\n\n") + "\n\n")
		m.viewport.GotoBottom()
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

type responseMsg string

func (m model) sendToLLM(message string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Get tools from MCP server
		tools, err := m.mcpClient.GetTools(ctx)
		if err != nil {
			return errMsg(fmt.Errorf("failed to get tools: %w", err))
		}

		// Send message to LLM
		response, err := m.llmClient.SendMessage(ctx, message, tools)
		if err != nil {
			return errMsg(fmt.Errorf("LLM error: %w", err))
		}

		// Handle tool calls if any
		if len(response.ToolCalls) > 0 {
			toolResults := make([]string, 0, len(response.ToolCalls))
			for _, toolCall := range response.ToolCalls {
				result, err := m.mcpClient.CallTool(ctx, toolCall.ToolName, toolCall.Arguments)
				if err != nil {
					return errMsg(fmt.Errorf("tool call error: %w", err))
				}
				toolResults = append(toolResults, result)
			}

			// Get final response from LLM with tool results
			finalResponse, err := m.llmClient.SendMessage(ctx, fmt.Sprintf("Tool results: %v", toolResults), nil)
			if err != nil {
				return errMsg(fmt.Errorf("LLM error after tool calls: %w", err))
			}
			return responseMsg(finalResponse.Content)
		}

		return responseMsg(response.Content)
	}
}

func (m model) View() string {
	return fmt.Sprintf(
		"%s\n\n%s",
		m.viewport.View(),
		m.textarea.View(),
	) + "\n\n(ctrl+c to quit)"
}

func main() {
	// Check for API key
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		// Try Gemini
		apiKey = os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			fmt.Println("Error: Please set ANTHROPIC_API_KEY or GEMINI_API_KEY environment variable")
			os.Exit(1)
		}
		// Use Gemini
		fmt.Println("Using Gemini API (not yet implemented)")
		os.Exit(1)
	}

	// Initialize Claude client
	claudeClient := NewClaudeClient(apiKey)

	// Initialize MCP client
	mcpClient, err := NewMCPClient()
	if err != nil {
		fmt.Printf("Error initializing MCP client: %v\n", err)
		os.Exit(1)
	}
	defer mcpClient.Close()

	// Create and run Bubble Tea program
	p := tea.NewProgram(
		initialModel(claudeClient, mcpClient),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
