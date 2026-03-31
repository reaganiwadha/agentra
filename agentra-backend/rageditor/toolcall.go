package rageditor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/tmc/langchaingo/llms"
	langopenai "github.com/tmc/langchaingo/llms/openai"
)

func InvokeToolLoop(ctx context.Context, req ToolInvocationRequest) (ToolInvocationResult, []domain.HighlightJobTrace, error) {
	traces := make([]domain.HighlightJobTrace, 0, 12)
	if req.Model.ProviderType != "" && req.Model.ProviderType != domain.ProviderOpenAICompat {
		err := fmt.Errorf("tool calling is unsupported for provider type %q", req.Model.ProviderType)
		traces = append(traces, newTrace(req.JobID, "llm.unsupported", "Configured provider does not support LangChainGo tool calling in this path.", map[string]any{
			"provider_type": req.Model.ProviderType,
			"error":         err.Error(),
		}))
		return ToolInvocationResult{}, traces, err
	}

	model, err := langopenai.New(
		langopenai.WithToken(req.Model.APIKey),
		langopenai.WithModel(req.Model.ModelName),
		langopenai.WithBaseURL(strings.TrimRight(req.Model.BaseURL, "/")),
	)
	if err != nil {
		traces = append(traces, newTrace(req.JobID, "llm.failed", "Could not initialize LangChainGo model for tool calling.", map[string]any{
			"error": err.Error(),
		}))
		return ToolInvocationResult{}, traces, err
	}

	tools := make([]llms.Tool, 0, len(req.Tools))
	toolNames := make([]string, 0, len(req.Tools))
	handlers := make(map[string]ToolHandler, len(req.Tools))
	for _, tool := range req.Tools {
		tools = append(tools, tool.Definition)
		if tool.Definition.Function != nil {
			toolNames = append(toolNames, tool.Definition.Function.Name)
			handlers[tool.Definition.Function.Name] = tool.Handle
		}
	}

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, req.SystemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, req.UserPrompt),
	}

	traces = append(traces, newTrace(req.JobID, "llm.request", "Submitting tool-loop request to editor model.", map[string]any{
		"provider_name":  req.ProviderName,
		"provider_type":  req.Model.ProviderType,
		"model_name":     req.Model.ModelName,
		"system_prompt":  req.SystemPrompt,
		"user_prompt":    req.UserPrompt,
		"prompt_visible": req.PromptVisible,
		"tool_names":     toolNames,
		"tool_choice":    req.ToolChoice,
		"max_iterations": req.MaxIterations,
	}))

	callOpts := []llms.CallOption{
		llms.WithTools(tools),
		llms.WithMaxTokens(req.MaxTokens),
	}
	if req.ToolChoice != nil {
		callOpts = append(callOpts, llms.WithToolChoice(req.ToolChoice))
	}

	maxIterations := req.MaxIterations
	if maxIterations < 1 {
		maxIterations = 4
	}

	result := ToolInvocationResult{
		ToolCalls:   make([]llms.ToolCall, 0),
		ToolOutputs: make([][]byte, 0),
	}

	for iteration := 1; iteration <= maxIterations; iteration++ {
		resp, err := model.GenerateContent(ctx, messages, callOpts...)
		if err != nil {
			traces = append(traces, newTrace(req.JobID, "llm.failed", "Editor model tool-loop request failed.", map[string]any{
				"iteration": iteration,
				"error":     err.Error(),
			}))
			return result, traces, err
		}
		if resp == nil || len(resp.Choices) == 0 || resp.Choices[0] == nil {
			err := fmt.Errorf("editor model returned no choices")
			traces = append(traces, newTrace(req.JobID, "llm.failed", "Editor model tool-loop returned no choices.", map[string]any{
				"iteration": iteration,
				"error":     err.Error(),
			}))
			return result, traces, err
		}

		choice := resp.Choices[0]
		result.AssistantContent = choice.Content
		result.ReasoningContent = choice.ReasoningContent
		traces = append(traces, newTrace(req.JobID, "llm.response", "Editor model returned a tool-loop response.", map[string]any{
			"iteration":         iteration,
			"content":           choice.Content,
			"reasoning_content": choice.ReasoningContent,
			"tool_calls":        choice.ToolCalls,
			"stop_reason":       choice.StopReason,
		}))

		if len(choice.ToolCalls) == 0 {
			result.TerminationReason = "stopped_tool_calling"
			result.IterationsUsed = iteration
			traces = append(traces, newTrace(req.JobID, "llm.loop.finished", "Editor model stopped requesting tools.", map[string]any{
				"iteration":         iteration,
				"content":           choice.Content,
				"reasoning_content": choice.ReasoningContent,
				"tool_call_count":   len(result.ToolCalls),
			}))
			return result, traces, nil
		}

		assistantParts := make([]llms.ContentPart, 0, len(choice.ToolCalls))
		toolResponseMessages := make([]llms.MessageContent, 0, len(choice.ToolCalls))
		for _, toolCall := range choice.ToolCalls {
			assistantParts = append(assistantParts, toolCall)
			result.ToolCalls = append(result.ToolCalls, toolCall)

			traces = append(traces, newTrace(req.JobID, "tool_call.selected", "Selected tool call from editor model response.", map[string]any{
				"iteration": iteration,
				"id":        toolCall.ID,
				"type":      toolCall.Type,
				"name":      toolCall.FunctionCall.Name,
				"arguments": toolCall.FunctionCall.Arguments,
			}))

			handler, ok := handlers[toolCall.FunctionCall.Name]
			if !ok || handler == nil {
				err := fmt.Errorf("no registered handler for tool %q", toolCall.FunctionCall.Name)
				traces = append(traces, newTrace(req.JobID, "tool_call.unhandled", "No registered handler exists for the selected tool.", map[string]any{
					"iteration": iteration,
					"name":      toolCall.FunctionCall.Name,
					"error":     err.Error(),
				}))
				return result, traces, err
			}

			output, err := handler(ctx, toolCall.FunctionCall.Arguments)
			if err != nil {
				traces = append(traces, newTrace(req.JobID, "tool_call.handler_failed", "Registered tool handler failed.", map[string]any{
					"iteration": iteration,
					"name":      toolCall.FunctionCall.Name,
					"arguments": toolCall.FunctionCall.Arguments,
					"error":     err.Error(),
				}))
				return result, traces, err
			}

			outputJSON, err := json.Marshal(output)
			if err != nil {
				traces = append(traces, newTrace(req.JobID, "tool_call.handler_failed", "Could not serialize tool handler output.", map[string]any{
					"iteration": iteration,
					"name":      toolCall.FunctionCall.Name,
					"error":     err.Error(),
				}))
				return result, traces, err
			}

			result.ToolOutputs = append(result.ToolOutputs, outputJSON)
			traces = append(traces, newTrace(req.JobID, "tool_call.handled", "Tool call arguments were normalized by the registered handler.", map[string]any{
				"iteration": iteration,
				"name":      toolCall.FunctionCall.Name,
				"output":    json.RawMessage(outputJSON),
			}))
			toolResponseMessages = append(toolResponseMessages, llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: toolCall.ID,
						Name:       toolCall.FunctionCall.Name,
						Content:    string(outputJSON),
					},
				},
			})
		}
		messages = append(messages, llms.MessageContent{
			Role:  llms.ChatMessageTypeAI,
			Parts: assistantParts,
		})
		messages = append(messages, toolResponseMessages...)
	}

	traces = append(traces, newTrace(req.JobID, "llm.loop.max_iterations", "Editor model hit the tool-loop iteration limit.", map[string]any{
		"max_iterations":  maxIterations,
		"tool_call_count": len(result.ToolCalls),
	}))
	result.TerminationReason = "max_iterations"
	result.IterationsUsed = maxIterations
	return result, traces, nil
}
