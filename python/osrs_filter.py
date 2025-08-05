"""
OSRS Item Filtering System

This module provides advanced filtering capabilities for OSRS items,
replicating the functionality of the web-based filtering interface.
"""

import requests
import pandas as pd
import numpy as np
from scipy import stats
import json
import time
import random
from datetime import datetime, timezone
from typing import Dict, List, Optional, Union


class OSRSItemFilter:
    """
    Advanced filtering system for OSRS items, similar to the web interface.
    Allows filteri        if volume_1h_min is not None:
            # Check if volume columns exist, if not, skip this filter
            if 'bought_volume_1h' in df.columns and 'sold_volume_1h' in df.columns:
                # Filter by items where BOTH bought and sold 1h volume are above threshold
                df = df[(df['bought_volume_1h'] >= volume_1h_min) & (df['sold_volume_1h'] >= volume_1h_min)]
                if verbose:
                    print(f"After both bought and sold 1h volume >= {volume_1h_min:,}: {len(df)} items")
            else:
                if verbose:
                    print(f"⚠️  Volume filtering skipped - volume data not loaded yet. Use load_volume_data() first.")

        if volume_24h_min is not None:
            # Check if volume columns exist, if not, skip this filter
            if 'bought_volume_24h' in df.columns and 'sold_volume_24h' in df.columns:
                # Filter by combined 24h volume (high + low)
                df = df[(df['bought_volume_24h'] >= volume_24h_min) & (df['sold_volume_24h'] >= volume_24h_min)]
                if verbose:
                    print(f"After both bought and sold 24h volume >= {volume_24h_min:,}: {len(df)} items")
            else:
                if verbose:
                    print(f"⚠️  24h volume filtering skipped - volume data not loaded yet. Use load_volume_data() first.")us criteria and computing derived metrics.
    """

    def __init__(self, base_url="https://prices.runescape.wiki/api/v1/osrs",
                 user_agent="osrs-flipping-analysis - educational project"):
        self.base_url = base_url
        self.headers = {
            'User-Agent': user_agent,
            'Accept': 'application/json'
        }
        self.df = None  # Single underlying dataframe

    def make_api_request(self, endpoint: str, params: Optional[Dict] = None) -> Dict:
        """
        Make a request to the OSRS pricing API with proper headers.

        Args:
            endpoint: API endpoint (e.g., 'latest', 'mapping', '5m')
            params: Optional query parameters

        Returns:
            JSON response as dictionary
        """
        url = f"{self.base_url}/{endpoint}"

        try:
            response = requests.get(url, headers=self.headers, params=params)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            print(f"Error making API request to {url}: {e}")
            return {}

    def get_latest_prices(self, item_id: Optional[int] = None) -> Dict:
        """Get latest high/low prices for all items or a specific item."""
        params = {'id': item_id} if item_id else None
        return self.make_api_request('latest', params)

    def get_item_mapping(self) -> List[Dict]:
        """Get mapping of item IDs to names and metadata."""
        return self.make_api_request('mapping')

    def get_timeseries(self, item_id: int, timestep: str = '5m') -> Dict:
        """
        Get timeseries data for a specific item.

        Args:
            item_id: Item ID to get timeseries for
            timestep: Time interval ('5m', '1h', '6h', '24h')

        Returns:
            Timeseries data dictionary
        """
        params = {'id': item_id, 'timestep': timestep}
        return self.make_api_request('timeseries', params)

    def calculate_volume_metrics(self, item_id: int, add_jitter: bool = True) -> Dict:
        """
        Calculate trading volume metrics for an item over different time periods.

        Args:
            item_id: Item ID to analyze
            add_jitter: Whether to add random delay to avoid API rate limiting

        Returns:
            Dictionary with volume metrics
        """
        # Add random jitter to avoid overwhelming the API
        if add_jitter:
            jitter = random.uniform(0.3, 0.9)  # 10ms to 500ms delay
            time.sleep(jitter)

        # Get 5-minute data (provides most granular data for recent activity)
        timeseries_5m = self.get_timeseries(item_id, '5m')

        # Get 24-hour data (provides data for longer timeframes)
        timeseries_24h = self.get_timeseries(item_id, '24h')

        if not timeseries_5m or 'data' not in timeseries_5m:
            return {}

        data_5m = timeseries_5m['data']
        if not data_5m:
            return {}

        data_24h = timeseries_24h.get('data', []) if timeseries_24h and 'data' in timeseries_24h else []

        # Convert to DataFrame for easier analysis
        df_5m = pd.DataFrame(data_5m)
        df_5m['timestamp'] = pd.to_datetime(df_5m['timestamp'], unit='s', utc=True)

        df_24h = pd.DataFrame(data_24h) if data_24h else pd.DataFrame()
        if not df_24h.empty:
            df_24h['timestamp'] = pd.to_datetime(df_24h['timestamp'], unit='s', utc=True)

        # Current time for calculations
        current_time = pd.Timestamp.now(tz='UTC')

        # Calculate time boundaries for 5-minute data
        one_hour_ago = current_time - pd.Timedelta(hours=1)
        twenty_four_hours_ago = current_time - pd.Timedelta(hours=24)
        twenty_minutes_ago = current_time - pd.Timedelta(minutes=20)

        # Calculate time boundaries for 24-hour data
        one_week_ago = current_time - pd.Timedelta(days=7)
        one_month_ago = current_time - pd.Timedelta(days=30)

        # Filter data for different time periods from 5m data
        last_hour = df_5m[df_5m['timestamp'] >= one_hour_ago]
        last_24_hours = df_5m[df_5m['timestamp'] >= twenty_four_hours_ago]
        last_20_minutes = df_5m[df_5m['timestamp'] >= twenty_minutes_ago]

        # Filter data for different time periods from 24h data
        last_week = df_24h[df_24h['timestamp'] >= one_week_ago] if not df_24h.empty else pd.DataFrame()
        last_month = df_24h[df_24h['timestamp'] >= one_month_ago] if not df_24h.empty else pd.DataFrame()

        # Calculate price trends
        def determine_price_trend(price_series):
            """Helper function to determine if prices are increasing, decreasing, or flat"""
            if len(price_series) < 3:  # Need at least 3 points for a meaningful trend
                return "flat"

            # Use linear regression to determine trend
            y = price_series.values
            x = np.arange(len(y))

            if len(y) == 0:
                return "flat"

            # Calculate slope using linear regression
            slope, _, _, _, _ = stats.linregress(x, y)

            # Set thresholds for determining trend significance
            # Calculate percentage change over the period
            if len(y) > 1:
                pct_change = (y[-1] - y[0]) / y[0] * 100
            else:
                pct_change = 0

            # Determine trend based on slope and percent change
            if abs(pct_change) < 1.0:  # Less than 1% change is considered flat
                return "flat"
            elif slope > 0:
                return "increasing"
            else:
                return "decreasing"

        # Calculate 1h trends
        bought_price_trend_1h = "flat"
        sold_price_trend_1h = "flat"

        if not last_hour.empty and len(last_hour) >= 3:
            # Sort by timestamp to ensure chronological order
            last_hour_sorted = last_hour.sort_values('timestamp')
            bought_price_trend_1h = determine_price_trend(last_hour_sorted['avgHighPrice'])
            sold_price_trend_1h = determine_price_trend(last_hour_sorted['avgLowPrice'])

        # Calculate 24h trends
        bought_price_trend_24h = "flat"
        sold_price_trend_24h = "flat"

        if not last_24_hours.empty and len(last_24_hours) >= 3:
            # Sort by timestamp to ensure chronological order
            last_24_hours_sorted = last_24_hours.sort_values('timestamp')
            bought_price_trend_24h = determine_price_trend(last_24_hours_sorted['avgHighPrice'])
            sold_price_trend_24h = determine_price_trend(last_24_hours_sorted['avgLowPrice'])

        # Calculate 1w trends
        bought_price_trend_1w = "flat"
        sold_price_trend_1w = "flat"

        if not last_week.empty and len(last_week) >= 3:
            # Sort by timestamp to ensure chronological order
            last_week_sorted = last_week.sort_values('timestamp')
            bought_price_trend_1w = determine_price_trend(last_week_sorted['avgHighPrice'])
            sold_price_trend_1w = determine_price_trend(last_week_sorted['avgLowPrice'])

        # Calculate 1m trends
        bought_price_trend_1m = "flat"
        sold_price_trend_1m = "flat"

        if not last_month.empty and len(last_month) >= 3:
            # Sort by timestamp to ensure chronological order
            last_month_sorted = last_month.sort_values('timestamp')
            bought_price_trend_1m = determine_price_trend(last_month_sorted['avgHighPrice'])
            sold_price_trend_1m = determine_price_trend(last_month_sorted['avgLowPrice'])

        # Calculate volume metrics
        metrics = {
            'item_id': item_id,

            # Last 20 minutes metrics
            'bought_volume_20m': last_20_minutes['highPriceVolume'].sum() if not last_20_minutes.empty else 0,
            'sold_volume_20m': last_20_minutes['lowPriceVolume'].sum() if not last_20_minutes.empty else 0,
            'avg_bought_price_20m': last_20_minutes['avgHighPrice'].mean() if not last_20_minutes.empty else 0,
            'avg_sold_price_20m': last_20_minutes['avgLowPrice'].mean() if not last_20_minutes.empty else 0,
            'avg_margin_gp_20m': (last_20_minutes['avgHighPrice'] - last_20_minutes['avgLowPrice']).mean() if not last_20_minutes.empty else 0,

            # Last hour metrics
            'bought_volume_1h': last_hour['highPriceVolume'].sum() if not last_hour.empty else 0,
            'sold_volume_1h': last_hour['lowPriceVolume'].sum() if not last_hour.empty else 0,
            'avg_bought_price_1h': last_hour['avgHighPrice'].mean() if not last_hour.empty else 0,
            'avg_sold_price_1h': last_hour['avgLowPrice'].mean() if not last_hour.empty else 0,
            'avg_margin_gp_1h': (last_hour['avgHighPrice'] - last_hour['avgLowPrice']).mean() if not last_hour.empty else 0,
            'bought_price_trend_1h': bought_price_trend_1h,
            'sold_price_trend_1h': sold_price_trend_1h,

            # Last 24 hours metrics
            'bought_volume_24h': last_24_hours['highPriceVolume'].sum() if not last_24_hours.empty else 0,
            'sold_volume_24h': last_24_hours['lowPriceVolume'].sum() if not last_24_hours.empty else 0,
            'avg_bought_price_24h': last_24_hours['avgHighPrice'].mean() if not last_24_hours.empty else 0,
            'avg_sold_price_24h': last_24_hours['avgLowPrice'].mean() if not last_24_hours.empty else 0,
            'avg_margin_gp_24h': (last_24_hours['avgHighPrice'] - last_24_hours['avgLowPrice']).mean() if not last_24_hours.empty else 0,
            'bought_price_trend_24h': bought_price_trend_24h,
            'sold_price_trend_24h': sold_price_trend_24h,

            # Last week metrics
            'bought_price_trend_1w': bought_price_trend_1w,
            'sold_price_trend_1w': sold_price_trend_1w,

            # Last month metrics
            'bought_price_trend_1m': bought_price_trend_1m,
            'sold_price_trend_1m': sold_price_trend_1m
        }

        return metrics

    def create_item_mapping_df(self) -> pd.DataFrame:
        """Create a pandas DataFrame with item mapping data."""
        mapping_data = self.get_item_mapping()
        if not mapping_data:
            return pd.DataFrame()

        df = pd.DataFrame(mapping_data)
        return df

    def create_latest_prices_df(self) -> pd.DataFrame:
        """Create a pandas DataFrame with latest price data."""
        prices_data = self.get_latest_prices()
        if not prices_data:
            return pd.DataFrame()

        # Convert to list of dictionaries for DataFrame creation
        price_records = []
        for item_id, price_info in prices_data.get('data', {}).items():
            record = {
                'item_id': int(item_id),
                'bought_price': price_info.get('high'),
                'last_bought_time': price_info.get('highTime'),
                'sold_price': price_info.get('low'),
                'last_sold_time': price_info.get('lowTime')
            }
            price_records.append(record)

        df = pd.DataFrame(price_records)

        # Convert timestamps to datetime with UTC timezone
        if not df.empty:
            df['last_bought_time'] = pd.to_datetime(df['last_bought_time'], unit='s', errors='coerce', utc=True)
            df['last_sold_time'] = pd.to_datetime(df['last_sold_time'], unit='s', errors='coerce', utc=True)

        return df

    def merge_prices_with_items(self, prices_df: pd.DataFrame, items_df: pd.DataFrame) -> pd.DataFrame:
        """Merge price data with item information."""
        if prices_df.empty or items_df.empty:
            return pd.DataFrame()

        # Ensure item_id columns have same type
        prices_df = prices_df.copy()
        items_df = items_df.copy()
        prices_df['item_id'] = prices_df['item_id'].astype(int)
        items_df['id'] = items_df['id'].astype(int)

        # rename items_df 'limit' column to 'buy_limit'
        if 'limit' in items_df.columns:
            items_df = items_df.rename(columns={'limit': 'buy_limit'})

        merged = prices_df.merge(items_df, left_on='item_id', right_on='id', how='left')
        return merged

    def load_data(self, force_reload=False):
        """Load and merge item and price data"""
        if not force_reload and self.df is not None:
            print("Data already loaded. Use force_reload=True to refresh.")
            return

        print("Loading data for filtering...")
        items_df = self.create_item_mapping_df()
        prices_df = self.create_latest_prices_df()

        if not items_df.empty and not prices_df.empty:
            self.df = self.merge_prices_with_items(prices_df, items_df)
            self.compute_derived_columns()
            print(f"✅ Loaded {len(self.df)} items with price data")
        else:
            print("❌ Failed to load data")
            self.df = None

    def compute_derived_columns(self):
        """Compute additional columns for analysis"""
        if self.df is None or self.df.empty:
            return

        # Basic price metrics
        self.df['margin_gp'] = self.df['bought_price'] - self.df['sold_price'].round(0)
        self.df['margin_pct'] = ((self.df['bought_price'] - self.df['sold_price']) / self.df['sold_price'] * 100).round(2)

        # Risk-adjusted opportunity score (flip efficiency)
        # Combines margin potential with volume liquidity
        if 'sold_volume_1h' in self.df.columns and 'bought_volume_1h' in self.df.columns:
            self.df['flip_efficiency'] = (
                self.df['margin_gp'] *
                (self.df['sold_volume_1h'] + self.df['bought_volume_1h']) / 2
            )
        else:
            # Initialize with 0 if volume data not available yet
            self.df['flip_efficiency'] = 0

        self.df['bought_time_rel'] = self.format_relative_time(self.df['last_bought_time'])
        self.df['sold_time_rel'] = self.format_relative_time(self.df['last_sold_time'])

        print("✅ Computed derived columns")

    def load_volume_data(self, item_ids: List[int] = None, max_items: int = 50):
        """
        Enrich the underlying dataframe with volume data from timeseries API.

        Args:
            item_ids: Specific item IDs to get volume for (if None, uses top items by margin)
            max_items: Maximum number of items to fetch volume data for (API rate limiting)
        """
        if self.df is None or self.df.empty:
            print("No data available. Please load data first.")
            return

        # If no specific items requested, use ever item_id in the datafraem
        if item_ids is None:
            item_ids = self.df['item_id'].head(max_items).tolist()

        print(f"Fetching volume data for {len(item_ids)} items...")

        # Initialize volume columns
        volume_columns = [
            'bought_volume_20m', 'sold_volume_20m',
            'avg_bought_price_20m', 'avg_sold_price_20m', 'avg_margin_gp_20m',
            'bought_volume_1h', 'sold_volume_1h',
            'avg_bought_price_1h', 'avg_sold_price_1h', 'avg_margin_gp_1h',
            'bought_volume_24h', 'sold_volume_24h',
            'avg_bought_price_24h', 'avg_sold_price_24h', 'avg_margin_gp_24h',
            'bought_price_trend_1h', 'sold_price_trend_1h',
            'bought_price_trend_24h', 'sold_price_trend_24h',
            'bought_price_trend_1w', 'sold_price_trend_1w',
            'bought_price_trend_1m', 'sold_price_trend_1m'
        ]

        for col in volume_columns:
            if col.startswith('bought_price_trend') or col.startswith('sold_price_trend'):
                self.df[col] = 'flat'  # Initialize trend columns as 'flat'
            else:
                self.df[col] = 0.0  # Initialize volume, price, and margin columns as 0

        # Fetch volume data for each item
        successful_fetches = 0
        for i, item_id in enumerate(item_ids):
            try:
                volume_metrics = self.calculate_volume_metrics(item_id)

                if volume_metrics:
                    # Update the dataframe with volume metrics
                    item_mask = self.df['item_id'] == item_id
                    for col in volume_columns:
                        if col in volume_metrics:
                            self.df.loc[item_mask, col] = volume_metrics[col]

                    successful_fetches += 1

                    # Progress indicator
                    if (i + 1) % 10 == 0:
                        print(f"  Processed {i + 1}/{len(item_ids)} items...")

            except Exception as e:
                print(f"  Error fetching data for item {item_id}: {e}")
                continue

        print(f"✅ Successfully enriched {successful_fetches}/{len(item_ids)} items with volume data")

        # Recalculate flip_efficiency now that volume data is available
        if 'sold_volume_1h' in self.df.columns and 'bought_volume_1h' in self.df.columns:
            self.df['flip_efficiency'] = (
                self.df['margin_gp'] *
                (self.df['sold_volume_1h'] + self.df['bought_volume_1h']) / 2
            )

    def apply_filter(self,
                     buy_limit_min=None, buy_limit_max=None,
                     bought_price_min=None, bought_price_max=None,
                     sold_price_min=None, sold_price_max=None,
                     margin_min=None, margin_max=None,
                     margin_pct_min=None, margin_pct_max=None,
                     volume_1h_min=None, volume_24h_min=None,
                     members_only=None,
                     max_hours_since_update=None,
                     name_contains=None,
                     exclude_items=None,
                     verbose=True,
                     sort_by: str = ("last_bought_time", "desc"),
                     limit=0, # integer limit number of rows. Should apply before computing derived columns. 0 is implied to mean no limit
                    ):

        """
        Apply filters similar to the web interface and return a filtered copy of the data.
        The underlying dataframe remains unchanged.
        """

        if self.df is None or self.df.empty:
            if verbose:
                print("No data available for filtering. Loading data...")
            self.load_data()
            if self.df is None or self.df.empty:
                print("Failed to load data for filtering")
                return pd.DataFrame()

        # Work with a copy to preserve the original data
        df = self.df.copy()

        # Remove items without price data
        df = df.dropna(subset=['bought_price', 'sold_price'])

        if verbose:
            print(f"Starting with {len(df)} items with price data")

        # Apply filters
        if buy_limit_min is not None and 'buy_limit' in df.columns:
            df = df[df['buy_limit'] >= buy_limit_min]
            if verbose:
                print(f"After buy limit >= {buy_limit_min}: {len(df)} items")

        if buy_limit_max is not None and 'buy_limit' in df.columns:
            df = df[df['buy_limit'] <= buy_limit_max]
            if verbose:
                print(f"After buy limit <= {buy_limit_max}: {len(df)} items")

        if bought_price_min is not None:
            df = df[df['sold_price'] >= bought_price_min]
            if verbose:
                print(f"After buy price >= {bought_price_min:,}: {len(df)} items")

        if bought_price_max is not None:
            df = df[df['sold_price'] <= bought_price_max]
            if verbose:
                print(f"After buy price <= {bought_price_max:,}: {len(df)} items")

        if sold_price_min is not None:
            df = df[df['bought_price'] >= sold_price_min]
            if verbose:
                print(f"After sell price >= {sold_price_min:,}: {len(df)} items")

        if sold_price_max is not None:
            df = df[df['bought_price'] <= sold_price_max]
            if verbose:
                print(f"After sell price <= {sold_price_max:,}: {len(df)} items")

        if margin_min is not None:
            df = df[df['margin_gp'] >= margin_min]
            if verbose:
                print(f"After margin >= {margin_min:,} GP: {len(df)} items")

        if margin_max is not None:
            df = df[df['margin_gp'] <= margin_max]
            if verbose:
                print(f"After margin <= {margin_max:,} GP: {len(df)} items")

        if margin_pct_min is not None:
            df = df[df['margin_pct'] >= margin_pct_min]
            if verbose:
                print(f"After margin >= {margin_pct_min}%: {len(df)} items")

        if margin_pct_max is not None:
            df = df[df['margin_pct'] <= margin_pct_max]
            if verbose:
                print(f"After margin <= {margin_pct_max}%: {len(df)} items")

        if volume_1h_min is not None:
            # Filter by items where BOTH bought and sold 1h volume are above threshold
            df = df[(df['bought_volume_1h'] >= volume_1h_min) & (df['sold_volume_1h'] >= volume_1h_min)]
            if verbose:
                print(f"After both bought and sold 1h volume >= {volume_1h_min:,}: {len(df)} items")

        if volume_24h_min is not None:
            # Filter by combined 1h volume (high + low)
            df = df[(df['bought_volume_24h'] >= volume_24h_min) & (df['sold_volume_24h'] >= volume_24h_min)]
            if verbose:
                print(f"After 24h volume >= {volume_24h_min:,}: {len(df)} items")

        if members_only is not None and 'members' in df.columns:
            df = df[df['members'] == members_only]
            member_text = "members" if members_only else "F2P"
            if verbose:
                print(f"After {member_text} filter: {len(df)} items")

        if max_hours_since_update is not None:
            # Calculate cutoff time
            current_time = pd.Timestamp.now(tz='UTC')
            cutoff_time = current_time - pd.Timedelta(hours=max_hours_since_update)

            # Keep items where either last_bought_time OR last_sold_time is within the threshold
            # (both times could be NaT/null, so we need to handle that)
            recent_bought = (df['last_bought_time'].notna()) & (df['last_bought_time'] >= cutoff_time)
            recent_sold = (df['last_sold_time'].notna()) & (df['last_sold_time'] >= cutoff_time)

            df = df[recent_bought | recent_sold]
            if verbose:
                print(f"After max {max_hours_since_update}h since update: {len(df)} items")

        if name_contains is not None and 'name' in df.columns:
            df = df[df['name'].str.contains(name_contains, case=False, na=False)]
            if verbose:
                print(f"After name contains '{name_contains}': {len(df)} items")

        if exclude_items is not None and 'name' in df.columns:
            exclude_pattern = '|'.join(exclude_items)
            df = df[~df['name'].str.contains(exclude_pattern, case=False, na=False)]
            if verbose:
                print(f"After excluding items: {len(df)} items")

        # Implement sorting if provided
        if sort_by is not None:
            if isinstance(sort_by, tuple) and len(sort_by) == 2:
                sort_column, sort_direction = sort_by
                ascending = sort_direction.lower() == 'asc'

                if sort_column in df.columns:
                    df = df.sort_values(sort_column, ascending=ascending)
                    if verbose:
                        direction_text = "ascending" if ascending else "descending"
                        print(f"Sorted by {sort_column} ({direction_text})")
                elif verbose:
                    print(f"Warning: Sort column '{sort_column}' not found in data")
            elif isinstance(sort_by, str):
                # Default to descending if only column name provided
                if sort_by in df.columns:
                    df = df.sort_values(sort_by, ascending=False)
                    if verbose:
                        print(f"Sorted by {sort_by} (descending)")
                elif verbose:
                    print(f"Warning: Sort column '{sort_by}' not found in data")

        # Implement limit if provided
        if limit is not None and limit > 0:
            original_count = len(df)
            df = df.head(limit)
            if verbose:
                print(f"Limited to top {limit} items (from {original_count})")

        # update once filtered
        self.df = df
        return df

    def get_data(self) -> pd.DataFrame:
        """
        Get a copy of the underlying dataframe.

        Returns:
            Copy of the underlying dataframe, or empty DataFrame if no data loaded
        """
        if self.df is None:
            return pd.DataFrame()
        return self.df.copy()

    def has_data(self) -> bool:
        """
        Check if data has been loaded.

        Returns:
            True if data is available, False otherwise
        """
        return self.df is not None and not self.df.empty

    def format_relative_time(self, timestamp):
        """
        Format a timestamp into a human-readable relative time (e.g., '5m ago', '2h ago').

        Args:
            timestamp: A pandas Timestamp, datetime object, or Series of timestamps

        Returns:
            A string representing the relative time or Series of strings
        """
        # If it's a Series, apply this function to each element
        if isinstance(timestamp, pd.Series):
            return timestamp.apply(self.format_relative_time)

        # Handle single timestamp
        if pd.isna(timestamp):
            return "N/A"

        now = pd.Timestamp.now(tz='UTC')
        if timestamp.tzinfo is None:
            timestamp = timestamp.tz_localize('UTC')

        diff = now - timestamp

        # Get total seconds
        seconds = diff.total_seconds()

        # Convert to appropriate unit
        if seconds < 60:
            return "a minute ago"
        elif seconds < 3600:
            minutes = int(seconds / 60)
            return f"{minutes}m ago"
        elif seconds < 86400:
            hours = int(seconds / 3600)
            return f"{hours}h ago"
        elif seconds < 604800:  # 7 days
            days = int(seconds / 86400)
            return f"{days}d ago"
        elif seconds < 2592000:  # 30 days
            weeks = int(seconds / 604800)
            return f"{weeks}w ago"
        else:
            months = int(seconds / 2592000)
            return f"{months}mo ago"

    # def _get_trend_emoji(self, trend_value):
    #     """Convert trend text to emoji representation."""
    #     if pd.isna(trend_value):
    #         return "❓"
    #     elif trend_value == "increasing":
    #         return "↗️"
    #     elif trend_value == "decreasing":
    #         return "↘️"
    #     elif trend_value == "flat":
    #         return "➡️"
    #     else:
    #         return str(trend_value)

    # def _get_efficiency_emoji(self, efficiency_value):
    #     """
    #     Get color-coded emoji based on flip efficiency ratio using WoW rarity colors.

    #     Thresholds based on percentile analysis to make rarity tiers meaningful:
    #     - ⚪ Poor: bottom 50% (< ~344k)
    #     - 🟢 Uncommon: 50-80th percentile (~344k - 2.2M)
    #     - 🔵 Rare: 80-90th percentile (~2.2M - 8.4M)
    #     - 🟣 Epic: 90-97th percentile (~8.4M - 42M)
    #     - 🟠 Legendary: 97-99.5th percentile (~42M - 63M)
    #     - 🟡 Artifact: top 0.5% (≥ ~63M) - truly exclusive
    #     """
    #     if pd.isna(efficiency_value) or efficiency_value <= 0:
    #         return "⚫"  # Black for no data/invalid
    #     elif efficiency_value < 343663:
    #         return "⚪"  # White for poor efficiency (bottom 50%)
    #     elif efficiency_value < 2227089:
    #         return "🟢"  # Green for uncommon efficiency (50-80th percentile)
    #     elif efficiency_value < 8365947:
    #         return "🔵"  # Blue for rare efficiency (80-90th percentile)
    #     elif efficiency_value < 42004968:
    #         return "🟣"  # Purple for epic efficiency (90-97th percentile)
    #     elif efficiency_value < 63442321:
    #         return "🟠"  # Orange for legendary efficiency (97-99.5th percentile)
    #     else:
    #         return "🟡"  # Yellow for artifact/mythic efficiency (top 0.5%)

    def __repr__(self):
        """
        Return a string representation of the filtered data with key columns.

        Displays only the most relevant columns for flipping analysis in a formatted table.
        """
        if not self.has_data():
            return "No data available. Use load_data() first."

        # Select and order the most relevant columns
        display_columns = [
            'name', 'margin_gp', 'margin_pct', 'flip_efficiency', 'buy_limit',
            'sold_price', 'bought_price',
            'sold_volume_20m', 'bought_volume_20m',
            'avg_sold_price_20m', 'avg_bought_price_20m', 'avg_margin_gp_20m',
            'sold_volume_1h', 'bought_volume_1h',
            'avg_sold_price_1h', 'avg_bought_price_1h', 'avg_margin_gp_1h',
            'sold_price_trend_1h', 'bought_price_trend_1h',
            'sold_volume_24h', 'bought_volume_24h',
            'avg_sold_price_24h', 'avg_bought_price_24h', 'avg_margin_gp_24h',
            'sold_price_trend_24h', 'bought_price_trend_24h',
            'sold_price_trend_1w', 'bought_price_trend_1w',
            'sold_price_trend_1m', 'bought_price_trend_1m',
            'last_sold_time', 'last_bought_time',
        ]

        # Filter columns that exist in the dataframe
        available_columns = [col for col in display_columns if col in self.df.columns]

        if not available_columns:
            return "Data loaded but no display columns available."

        # Create a copy with only the display columns
        display_df = self.df[available_columns].copy()

        # Add relative time columns
        if 'last_bought_time' in display_df.columns:
            display_df['bought_time_rel'] = display_df['last_bought_time'].apply(self.format_relative_time)
            cols = list(display_df.columns)
            buy_idx = cols.index('bought_price')
            cols.insert(buy_idx + 1, 'bought_time_rel')
            cols.remove('bought_time_rel')
            display_df = display_df[cols]
            display_df = display_df.rename(columns={'last_bought_time': 'bought_time'})

        if 'last_sold_time' in display_df.columns:
            display_df['sold_time_rel'] = display_df['last_sold_time'].apply(self.format_relative_time)
            cols = list(display_df.columns)
            sell_idx = cols.index('sold_price')
            cols.insert(sell_idx + 1, 'sold_time_rel')
            cols.remove('sold_time_rel')
            display_df = display_df[cols]
            display_df = display_df.rename(columns={'last_sold_time': 'sold_time'})

        # Format timestamps to be more readable
        if 'bought_time' in display_df.columns:
            display_df['bought_time'] = display_df['bought_time'].dt.strftime('%Y-%m-%d %H:%M')
        if 'sold_time' in display_df.columns:
            display_df['sold_time'] = display_df['sold_time'].dt.strftime('%Y-%m-%d %H:%M')

        # Format numeric columns with proper formatting
        if 'margin_pct' in display_df.columns:
            display_df['margin_pct'] = display_df['margin_pct'].apply(lambda x: f"{x:.2f}%" if pd.notna(x) else "N/A")
        if 'margin_gp' in display_df.columns:
            display_df['margin_gp'] = display_df['margin_gp'].apply(lambda x: f"{int(x):,}" if pd.notna(x) else "N/A")
        if 'flip_efficiency' in display_df.columns:
            display_df['flip_efficiency'] = display_df['flip_efficiency'].apply(lambda x: f"{int(x):,}" if pd.notna(x) and x > 0 else "N/A")

        # Format price columns
        for col in ['bought_price', 'sold_price', 'avg_bought_price_20m', 'avg_sold_price_20m',
                    'avg_bought_price_1h', 'avg_sold_price_1h', 'avg_bought_price_24h', 'avg_sold_price_24h']:
            if col in display_df.columns:
                display_df[col] = display_df[col].apply(lambda x: f"{int(x):,}" if pd.notna(x) and x > 0 else "N/A")

        # Format margin columns
        for col in ['avg_margin_gp_20m', 'avg_margin_gp_1h', 'avg_margin_gp_24h']:
            if col in display_df.columns:
                display_df[col] = display_df[col].apply(lambda x: f"{int(x):,}" if pd.notna(x) and x > 0 else "N/A")

        # Format volume columns
        for col in ['bought_volume_1h', 'sold_volume_1h', 'bought_volume_24h', 'sold_volume_24h',
                    'bought_volume_20m', 'sold_volume_20m']:
            if col in display_df.columns:
                display_df[col] = display_df[col].apply(lambda x: f"{int(x):,}" if pd.notna(x) else "0")

        # Get row count
        row_count = len(display_df)

        # Create simplified representation
        legend = """
OSRS Price Semantics (counterintuitive):
   • sold_price = what you can BUY at (instant sell order fill price)
   • bought_price = what you can SELL at (instant buy order fill price)
   • margin_gp = bought_price - sold_price = potential profit per item

Trend Values: increasing, decreasing, flat
"""

        result = f"""{legend}
OSRSItemFilter Results ({row_count} items)
{display_df.to_string(index=False)}
        """
        return result

    def save(self, filepath, format='csv'):
        """
        Save the current dataframe to a file.

        Args:
            filepath: Path where the file will be saved
            format: File format - 'csv', 'json', or 'pickle'

        Returns:
            True if save was successful, False otherwise
        """
        if not self.has_data():
            print("No data available to save.")
            return False

        try:
            if format.lower() == 'csv':
                self.df.to_csv(filepath, index=False)
            elif format.lower() == 'json':
                self.df.to_json(filepath, orient='records', date_format='iso')
            elif format.lower() == 'pickle':
                self.df.to_pickle(filepath)
            else:
                print(f"Unsupported format: {format}. Use 'csv', 'json', or 'pickle'.")
                return False

            print(f"Data saved successfully to {filepath}")
            return True
        except Exception as e:
            print(f"Error saving data: {e}")
            return False

    def load_from_file(self, filepath, format='csv'):
        """
        Load dataframe from a file.

        Args:
            filepath: Path to the file to load
            format: File format - 'csv', 'json', or 'pickle'

        Returns:
            True if load was successful, False otherwise
        """
        try:
            if format.lower() == 'csv':
                df = pd.read_csv(filepath)

                # Convert timestamp columns back to datetime
                timestamp_cols = ['last_bought_time', 'last_sold_time']
                for col in timestamp_cols:
                    if col in df.columns:
                        df[col] = pd.to_datetime(df[col], utc=True)

            elif format.lower() == 'json':
                df = pd.read_json(filepath, orient='records')

                # Convert timestamp columns back to datetime
                timestamp_cols = ['last_bought_time', 'last_sold_time']
                for col in timestamp_cols:
                    if col in df.columns:
                        df[col] = pd.to_datetime(df[col], utc=True)

            elif format.lower() == 'pickle':
                df = pd.read_pickle(filepath)
            else:
                print(f"Unsupported format: {format}. Use 'csv', 'json', or 'pickle'.")
                return False

            self.df = df
            print(f"Loaded {len(df)} items from {filepath}")

            # Recompute derived columns to ensure consistency
            self.compute_derived_columns()
            return True
        except Exception as e:
            print(f"Error loading data from {filepath}: {e}")
            return False