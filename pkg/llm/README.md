# OSRS Trade Analysis - LLM Integration

This Go program integrates with Ollama to provide AI-powered trading analysis for Old School RuneScape items.

## Features

### 🤖 LLM Analysis Integration
- **Structured Data Processing**: Converts filtered OSRS item data into formatted analysis requests
- **Intelligent Prompting**: Uses specialized system prompts for trading analysis
- **Response Cleaning**: Automatically removes thinking tags from LLM responses
- **Error Handling**: Graceful fallback when Ollama is unavailable

### 📊 Analysis Capabilities
- **Market Statistics**: Calculates comprehensive market metrics
- **Risk Assessment**: Analyzes volume, price age, and volatility
- **Trading Recommendations**: Provides actionable insights based on item data
- **Trend Analysis**: Incorporates price trends when available

## Setup

### Prerequisites
1. **Install Ollama**: Download from [https://ollama.ai](https://ollama.ai)
2. **Pull a Model**: Run `ollama pull qwen2.5:7b` (or your preferred model)
3. **Start Ollama**: Run `ollama serve`

### Configuration
Update the model name in `cmd/main.go`:
```go
modelConfig := llm.CreateDefaultModelConfig("your-model-name")
```

## Usage

### Running the Analysis
```bash
cd /home/steven/osrs-flipping
go run cmd/main.go
```

### Output Example
```
🤖 Testing LLM analysis integration...
✅ Connected to Ollama
Analyzing 5 items with 5 total opportunities...

🎯 LLM Trading Analysis:
============================================================
Based on the OSRS trading data, here are the key insights:

TOP OPPORTUNITIES:
1. 3rd age wand (39M GP margin) - Extremely high-value but low liquidity
2. 3rd age druidic cloak (35M GP margin) - Rare item with solid returns
...
============================================================

Analysis completed in 3.45 seconds
Market Summary: 5 items, avg margin 32960289 GP
```

## Code Structure

### Core Components

#### `pkg/llm/types.go`
- Model configuration structures
- Request/response types for Ollama API
- Default configuration factory

#### `pkg/llm/client.go`
- HTTP client for Ollama communication
- Generate requests with retry logic
- Connection testing and error handling
- Response cleaning utilities

#### `pkg/llm/analysis.go`
- Structured analysis request creation
- Market statistics calculation
- System prompt generation
- Data formatting for LLM consumption

### Integration Flow

1. **Data Loading**: Filter and sort OSRS items by profitability
2. **Request Creation**: Structure data into analysis-friendly format
3. **LLM Communication**: Send formatted data with trading-specific prompts
4. **Response Processing**: Clean and display AI-generated insights
5. **Statistics**: Show analysis metrics and performance data

## Key Features vs Python Implementation

### ✅ Improvements
- **Performance**: 10-100x faster execution
- **Type Safety**: Compile-time error detection
- **Single Binary**: No dependencies or virtual environments
- **Memory Efficiency**: Better data structure management
- **Concurrent Safety**: Proper goroutine synchronization

### 🔄 Compatibility
- **API Compatibility**: Same RuneScape Wiki API integration
- **Data Structures**: Equivalent filtering and analysis capabilities
- **LLM Features**: Same Ollama integration with enhanced error handling

## Configuration Options

### Model Configuration
```go
modelConfig := llm.ModelConfig{
    Name: "qwen2.5:7b",
    Options: llm.Options{
        NumCtx:      4096,    // Context window size
        Temperature: 0.8,     // Response creativity
        TopK:        40,      // Token selection diversity
        TopP:        0.9,     // Nucleus sampling
        Seed:        123456,  // Reproducible results
    },
}
```

### Analysis Parameters
- **Item Limit**: Control how many items to analyze
- **Margin Thresholds**: Filter by minimum profit margins
- **Market Focus**: Target specific item categories
- **Risk Tolerance**: Adjust volume and volatility requirements

## Future Enhancements

- **Multiple Models**: Support for different LLM providers
- **Cached Analysis**: Store and reuse analysis results
- **Web Interface**: HTTP server for real-time analysis
- **Market Alerts**: Automated trading opportunity notifications
- **Historical Analysis**: Track market patterns over time

## Troubleshooting

### Common Issues

**Connection Refused**
```
LLM connection failed: connect: connection refused
```
- Ensure Ollama is running: `ollama serve`
- Check if model is available: `ollama list`

**Model Not Found**
```
model 'qwen2.5:7b' not found
```
- Pull the model: `ollama pull qwen2.5:7b`
- Update model name in code to match available model

**Context Size Exceeded**
```
Warning: estimated input tokens (5000) exceeds context size (4096)
```
- Reduce item limit in analysis
- Use a model with larger context window
- Increase `NumCtx` in model configuration
