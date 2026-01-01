// Recharts Theme Configuration for StockAI
// Use these theme values when rendering Recharts components

const getChartTheme = () => {
  const isDark = document.documentElement.classList.contains('dark');
  
  return {
    colors: {
      line: '#6366F1',           // accent-primary - main line color
      lineFill: 'rgba(99, 102, 241, 0.1)', // accent with transparency
      positive: '#10B981',       // gains, buy signals
      negative: '#EF4444',       // losses, sell signals
      warning: '#F59E0B',        // alerts, caution
      info: '#3B82F6',           // informational
      grid: isDark ? '#334155' : '#E2E8F0',
      text: isDark ? '#64748B' : '#94A3B8',
      textPrimary: isDark ? '#F8FAFC' : '#0F172A',
      background: isDark ? '#1E293B' : '#FFFFFF',
      tooltip: isDark ? '#1F2937' : '#FFFFFF',
    },
    
    // Common chart props for consistent styling
    chartProps: {
      margin: { top: 10, right: 30, left: 0, bottom: 0 },
    },
    
    // XAxis default styling
    xAxisProps: {
      axisLine: false,
      tickLine: false,
      tick: { fill: isDark ? '#64748B' : '#94A3B8', fontSize: 12 },
    },
    
    // YAxis default styling
    yAxisProps: {
      axisLine: false,
      tickLine: false,
      tick: { fill: isDark ? '#64748B' : '#94A3B8', fontSize: 12 },
      width: 60,
    },
    
    // CartesianGrid styling
    gridProps: {
      strokeDasharray: '3 3',
      stroke: isDark ? '#334155' : '#E2E8F0',
      vertical: false,
    },
    
    // Tooltip styling
    tooltipProps: {
      contentStyle: {
        backgroundColor: isDark ? '#1F2937' : '#FFFFFF',
        border: `1px solid ${isDark ? '#334155' : '#E2E8F0'}`,
        borderRadius: '8px',
        boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1)',
      },
      labelStyle: {
        color: isDark ? '#F8FAFC' : '#0F172A',
        fontWeight: 600,
      },
      itemStyle: {
        color: isDark ? '#CBD5E1' : '#475569',
      },
    },
  };
};

// Lightweight Charts configuration
const getLightweightChartsTheme = () => {
  const isDark = document.documentElement.classList.contains('dark');
  
  return {
    layout: {
      background: { type: 'solid', color: isDark ? '#1E293B' : '#FFFFFF' },
      textColor: isDark ? '#CBD5E1' : '#475569',
    },
    grid: {
      vertLines: { color: isDark ? '#334155' : '#E2E8F0' },
      horzLines: { color: isDark ? '#334155' : '#E2E8F0' },
    },
    crosshair: {
      mode: 0, // Normal
      vertLine: {
        color: isDark ? '#64748B' : '#94A3B8',
        width: 1,
        style: 2, // Dashed
        labelBackgroundColor: '#6366F1',
      },
      horzLine: {
        color: isDark ? '#64748B' : '#94A3B8',
        width: 1,
        style: 2,
        labelBackgroundColor: '#6366F1',
      },
    },
    rightPriceScale: {
      borderColor: isDark ? '#334155' : '#E2E8F0',
    },
    timeScale: {
      borderColor: isDark ? '#334155' : '#E2E8F0',
    },
  };
};

// Area series options for price charts
const getAreaSeriesOptions = () => ({
  topColor: 'rgba(99, 102, 241, 0.4)',
  bottomColor: 'rgba(99, 102, 241, 0.0)',
  lineColor: '#6366F1',
  lineWidth: 2,
});

// Candlestick series options
const getCandlestickOptions = () => ({
  upColor: '#10B981',
  downColor: '#EF4444',
  borderUpColor: '#10B981',
  borderDownColor: '#EF4444',
  wickUpColor: '#10B981',
  wickDownColor: '#EF4444',
});

// Export for use in other scripts
if (typeof window !== 'undefined') {
  window.StockAICharts = {
    getChartTheme,
    getLightweightChartsTheme,
    getAreaSeriesOptions,
    getCandlestickOptions,
  };
}
