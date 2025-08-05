#!/usr/bin/env python3
"""
OSRS Trading Analysis Service

Implements the service program flow from the todo's:
1. Look at the wide market and select 6 items to trade
2. Use LLM to analyze and provide structured recommendations
"""

import json
import os
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional

from llm.client import ModelConfig, get_chat_response, init_client, remove_thinking_tags
from utils.config import settings


class OSRSTradeAnalyzer:
    """LLM-powered OSRS trading analysis service."""

    def __init__(self, user_agent: str = "learning pandas with osrs - @sf1tzp"):
        self.user_agent = user_agent
        # Use smaller, more memory-efficient model
        self.model_config = ModelConfig.create_default("qwen3:14b")
        self.model_config.options.num_ctx = 9000  # Reasonable context window
        self.client = None

    def initialize_llm(self):
        """Initialize the LLM client connection."""
        if not self.client:
            self.client = init_client(settings.openai_api.url)
            print(f"✅ Connected to Ollama at {settings.openai_api.url}")

    def load_market_data(self) -> tuple[str, str]:
        """Load market data and signals for analysis."""
        # Load the current market data
        with open("/home/steven/osrs-flipping/over-here!.txt", "r") as f:
            market_data = f.read()

        # Load the signals documentation
        with open("/home/steven/osrs-flipping/signals.md", "r") as f:
            signals_data = f.read()

        return market_data, signals_data

    def filter_promising_items(self, market_data: str, max_items: int = 50) -> str:
        """Filter market data to focus on the most promising trading opportunities."""
        lines = market_data.split('\n')

        # Find the header and data lines
        header_found = False
        header_line = ""
        data_lines = []

        for line in lines:
            if "name margin_gp margin_pct" in line and not header_found:
                header_line = line
                header_found = True
                continue

            if header_found and line.strip() and not line.startswith('#') and not line.startswith('📊'):
                # Extract margin_gp (second column after name)
                parts = line.split()
                if len(parts) > 2:
                    try:
                        # Try to extract margin info to filter by
                        margin_gp_str = parts[1].replace(',', '')
                        margin_gp = float(margin_gp_str)
                        margin_pct_str = parts[2].replace('%', '')
                        margin_pct = float(margin_pct_str)

                        # Filter criteria: decent margin and manageable investment
                        if margin_gp >= 1000 and margin_pct >= 2.0:
                            data_lines.append((margin_gp, line))
                    except (ValueError, IndexError):
                        continue

        # Sort by margin_gp descending and take top items
        data_lines.sort(key=lambda x: x[0], reverse=True)
        top_items = [line for _, line in data_lines[:max_items]]

        # Reconstruct the filtered data with legend
        legend_lines = []
        for line in lines:
            if line.startswith('📊') or line.startswith('📈') or line.startswith('💡'):
                legend_lines.append(line)
                if line.startswith('💡'):  # Stop after the price semantics explanation
                    break

        result = '\n'.join(legend_lines) + '\n\n'
        result += f"🎯 OSRSItemFilter Results (Top {len(top_items)} items)\n"
        result += header_line + '\n'
        result += '\n'.join(top_items)

        return result

    def load_analysis_prompt(self) -> str:
        """Load the analysis prompt template."""
        with open("/home/steven/osrs-flipping/prompt.txt", "r") as f:
            return f.read()

    def analyze_trading_opportunities(self) -> str:
        """Use LLM to analyze current market data and recommend 6 trading opportunities."""
        self.initialize_llm()

        market_data, signals_data = self.load_market_data()
        analysis_prompt = self.load_analysis_prompt()

        # Filter to most promising items to reduce context size
        filtered_market_data = self.filter_promising_items(market_data)

        print(f"📊 Filtered the top promising items to analyze")

        # Create the system prompt
        system_prompt = """You are an expert OSRS trading analyst. Your task is to analyze current market data and recommend the top 6 trading opportunities.

Key requirements:
1. Select exactly 6 items for short-term trading (speed + margin + liquidity)
2. Ensure each recommendation includes strategy price points even if they exceed limits
3. Note the time "ago" that current margins were set
4. Use the exact format from the examples provided
5. Focus on items with good volume and reasonable margins (aim for 2.1% minimum)
6. Consider up to 5M investment per item
7. Prioritize speed and liquidity over pure margin percentage

Remember the counterintuitive OSRS pricing:
- sold_price = what you can BUY at
- bought_price = what you can SELL at
- margin_gp = bought_price - sold_price"""

        # Create the user prompt with current data
        user_prompt = f"""{analysis_prompt}

Current Market Data (Top 50 Opportunities):
{filtered_market_data}

Trading Signals Reference:
{signals_data}

Please analyze this data and provide your top 6 trading recommendations in the exact format shown in the examples."""

        messages = [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt}
        ]

        print(f"🤖 Analyzing market data with LLM...")
        response = get_chat_response(self.model_config, messages, self.client)

        # Remove thinking tags from the response
        clean_response = remove_thinking_tags(response.message.content)

        return clean_response or response.message.content

    def save_analysis(self, analysis: str, filename_prefix: str = "market-capture") -> str:
        """Save analysis to a timestamped file."""
        timestamp = datetime.now().strftime("%Y%m%d-%H%M")
        filename = f"{filename_prefix}-{timestamp}.txt"
        filepath = Path(filename)

        with open(filepath, "w") as f:
            f.write(f"OSRS Trading Analysis - {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n")
            f.write("=" * 60 + "\n\n")
            f.write(analysis)

        print(f"💾 Analysis saved to: {filepath}")
        return str(filepath)

    def extract_item_names(self, analysis: str) -> List[str]:
        """Extract item names from the analysis for follow-up monitoring."""
        lines = analysis.split('\n')
        items = []

        for line in lines:
            # Look for numbered recommendations in various formats
            if line.strip().startswith(('1.', '2.', '3.', '4.', '5.', '6.', '###')):
                # Handle different formats
                if '###' in line and '**' in line:
                    # Format: ### **1. Onyx**
                    import re
                    match = re.search(r'\*\*\d+\.\s*([^*]+)\*\*', line)
                    if match:
                        items.append(match.group(1).strip())
                elif line.strip().startswith(('1.', '2.', '3.', '4.', '5.', '6.')):
                    # Format: 1. Item name - description
                    parts = line.split(' - ', 1)
                    if len(parts) > 1:
                        item_part = parts[0]
                        # Remove the number and extra spaces
                        item_name = item_part.split('.', 1)[1].strip()
                        items.append(item_name)

        return items[:6]  # Ensure we only return up to 6 items

    def run_market_analysis(self) -> Dict:
        """Run the full market analysis workflow."""
        print("🚀 Starting OSRS Trading Analysis...")

        try:
            # Analyze trading opportunities
            analysis = self.analyze_trading_opportunities()

            # Save the analysis
            filepath = self.save_analysis(analysis)

            # Extract item names for monitoring
            selected_items = self.extract_item_names(analysis)

            result = {
                "timestamp": datetime.now().isoformat(),
                "analysis_file": filepath,
                "selected_items": selected_items,
                "analysis_preview": analysis[:500] + "..." if len(analysis) > 500 else analysis
            }

            print(f"✅ Analysis complete! Selected {len(selected_items)} items for monitoring.")
            print(f"📋 Selected items: {', '.join(selected_items)}")

            return result

        except Exception as e:
            print(f"❌ Analysis failed: {e}")
            raise


def main():
    """Main entry point for the trading analysis service."""
    analyzer = OSRSTradeAnalyzer()
    result = analyzer.run_market_analysis()

    # Print a summary
    print("\n" + "="*60)
    print("ANALYSIS SUMMARY")
    print("="*60)
    print(f"Timestamp: {result['timestamp']}")
    print(f"Analysis saved to: {result['analysis_file']}")
    print(f"Selected items: {result['selected_items']}")
    print("\nAnalysis preview:")
    print("-" * 40)
    print(result['analysis_preview'])


if __name__ == "__main__":
    main()
