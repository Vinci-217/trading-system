'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { formatCurrency, formatNumber } from '@/lib/utils';
import { TrendingUp, RefreshCw, TrendingDown } from 'lucide-react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';

interface Quote {
  symbol: string;
  symbol_name: string;
  price: number;
  change: number;
  change_percent: number;
  high: number;
  low: number;
  open: number;
  volume: number;
  amount: number;
}

interface KLine {
  timestamp: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

interface Stock {
  symbol: string;
  symbol_name: string;
  exchange: string;
  status: string;
}

export default function MarketPage() {
  const router = useRouter();
  const [selectedSymbol, setSelectedSymbol] = useState('600519');
  const [quote, setQuote] = useState<Quote | null>(null);
  const [klines, setKlines] = useState<KLine[]>([]);
  const [stocks, setStocks] = useState<Stock[]>([]);
  const [period, setPeriod] = useState('1d');

  useEffect(() => {
    const token = localStorage.getItem('token');
    if (!token) {
      router.push('/');
      return;
    }

    fetchStocks();
    fetchQuote();
  }, [router, selectedSymbol]);

  useEffect(() => {
    if (selectedSymbol) {
      fetchKLines();
    }
  }, [selectedSymbol, period]);

  const getAuthHeaders = () => ({
    'Authorization': `Bearer ${localStorage.getItem('token')}`,
    'Content-Type': 'application/json',
  });

  const fetchStocks = async () => {
    try {
      const symbols = ['600519', '000001', '600036', '000002', '600276', '000651', '600030', '000725', '600887', '002594', 'AAPL', 'GOOGL', 'MSFT'];
      const stockData = symbols.map((symbol, index) => ({
        symbol,
        symbol_name: getStockName(symbol),
        exchange: symbol.includes('AAPL') || symbol.includes('GOOGL') || symbol.includes('MSFT') ? 'NASDAQ' : 'SSE',
        status: 'ACTIVE'
      }));
      setStocks(stockData);
    } catch (error) {
      console.error('获取股票列表失败:', error);
    }
  };

  const getStockName = (symbol: string) => {
    const names: Record<string, string> = {
      '600519': '贵州茅台',
      '000001': '平安银行',
      '600036': '招商银行',
      '000002': '万 科Ａ',
      '600276': '恒瑞医药',
      '000651': '格力电器',
      '600030': '中信证券',
      '000725': '京东方Ａ',
      '600887': '伊利股份',
      '002594': '比亚迪',
      'AAPL': '苹果公司',
      'GOOGL': '谷歌公司',
      'MSFT': '微软公司',
    };
    return names[symbol] || symbol;
  };

  const fetchQuote = async () => {
    try {
      const response = await fetch(`http://localhost:8080/api/v1/quote/${selectedSymbol}`, {
        headers: getAuthHeaders(),
      });
      if (response.ok) {
        const data = await response.json();
        setQuote(data);
      }
    } catch (error) {
      console.error('获取行情失败:', error);
    }
  };

  const fetchKLines = async () => {
    try {
      const response = await fetch(`http://localhost:8080/api/v1/kline/${selectedSymbol}?period=${period}&limit=100`, {
        headers: getAuthHeaders(),
      });
      if (response.ok) {
        const data = await response.json();
        const klineData = (data.klines || []).map((k: any) => ({
          ...k,
          timestamp: new Date(k.timestamp).getTime(),
        }));
        setKlines(klineData);
      }
    } catch (error) {
      console.error('获取K线失败:', error);
    }
  };

  const formatVolume = (volume: number) => {
    if (volume >= 100000000) {
      return (volume / 100000000).toFixed(2) + '亿';
    } else if (volume >= 10000) {
      return (volume / 10000).toFixed(2) + '万';
    }
    return volume.toString();
  };

  const chartData = klines.map(k => ({
    time: new Date(k.timestamp).toLocaleDateString(),
    open: k.open,
    high: k.high,
    low: k.low,
    close: k.close,
    volume: k.volume,
  }));

  return (
    <div className="min-h-screen bg-gray-100">
      <nav className="bg-blue-600 text-white p-4">
        <div className="max-w-7xl mx-auto flex justify-between items-center">
          <div className="flex items-center gap-2">
            <TrendingUp className="h-6 w-6" />
            <span className="text-xl font-bold">行情中心</span>
          </div>
          <div className="flex items-center gap-4">
            <Button variant="secondary" size="sm" onClick={() => router.push('/trade')}>
              交易
            </Button>
            <Button variant="secondary" size="sm" onClick={() => router.push('/account')}>
              账户
            </Button>
          </div>
        </div>
      </nav>

      <div className="max-w-7xl mx-auto p-4">
        <div className="grid grid-cols-4 gap-4">
          <Card className="col-span-1">
            <CardHeader>
              <CardTitle className="text-lg">股票列表</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-1 max-h-[600px] overflow-y-auto">
                {stocks.map(stock => (
                  <button
                    key={stock.symbol}
                    onClick={() => setSelectedSymbol(stock.symbol)}
                    className={`w-full p-2 text-left rounded text-sm ${
                      selectedSymbol === stock.symbol ? 'bg-blue-100' : 'hover:bg-gray-100'
                    }`}
                  >
                    <div className="font-medium">{stock.symbol_name}</div>
                    <div className="text-gray-500 text-xs">{stock.symbol}</div>
                  </button>
                ))}
              </div>
            </CardContent>
          </Card>

          <div className="col-span-3 space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="flex justify-between items-center">
                  <div>
                    <span className="text-2xl">{quote?.symbol_name}</span>
                    <span className="ml-2 text-gray-500">({selectedSymbol})</span>
                  </div>
                  <div className="flex items-center gap-4">
                    <span className={`text-3xl ${quote?.change >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                      {formatCurrency(quote?.price || 0)}
                    </span>
                    <div className={`text-right ${quote?.change >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                      <div className="flex items-center">
                        {quote?.change >= 0 ? <TrendingUp className="h-4 w-4 mr-1" /> : <TrendingDown className="h-4 w-4 mr-1" />}
                        <span>{formatCurrency(Math.abs(quote?.change || 0))}</span>
                      </div>
                      <div>{formatNumber((quote?.change_percent || 0), 2)}%</div>
                    </div>
                  </div>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-5 gap-4 text-sm">
                  <div>
                    <span className="text-gray-500">今开</span>
                    <p className="font-medium">{formatCurrency(quote?.open || 0)}</p>
                  </div>
                  <div>
                    <span className="text-gray-500">最高</span>
                    <p className="font-medium text-red-500">{formatCurrency(quote?.high || 0)}</p>
                  </div>
                  <div>
                    <span className="text-gray-500">最低</span>
                    <p className="font-medium text-green-500">{formatCurrency(quote?.low || 0)}</p>
                  </div>
                  <div>
                    <span className="text-gray-500">成交量</span>
                    <p className="font-medium">{formatVolume(quote?.volume || 0)}</p>
                  </div>
                  <div>
                    <span className="text-gray-500">成交额</span>
                    <p className="font-medium">{formatCurrency((quote?.amount || 0) / 100000000)}亿</p>
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between">
                <CardTitle className="text-lg">K线图</CardTitle>
                <div className="flex gap-2">
                  {['1m', '5m', '15m', '1h', '1d', '1w'].map(p => (
                    <Button
                      key={p}
                      variant={period === p ? 'default' : 'outline'}
                      size="sm"
                      onClick={() => setPeriod(p)}
                    >
                      {p}
                    </Button>
                  ))}
                  <Button variant="outline" size="sm" onClick={fetchKLines}>
                    <RefreshCw className="h-4 w-4" />
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                <div className="h-[400px]">
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="time" fontSize={12} />
                      <YAxis domain={['auto', 'auto']} fontSize={12} />
                      <Tooltip />
                      <Line type="monotone" dataKey="close" stroke="#2563eb" dot={false} />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-lg">分时数据</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b">
                        <th className="text-left p-2">时间</th>
                        <th className="text-right p-2">开盘</th>
                        <th className="text-right p-2">最高</th>
                        <th className="text-right p-2">最低</th>
                        <th className="text-right p-2">收盘</th>
                        <th className="text-right p-2">成交量</th>
                      </tr>
                    </thead>
                    <tbody>
                      {chartData.slice(-10).reverse().map((k, index) => (
                        <tr key={index} className="border-b hover:bg-gray-50">
                          <td className="p-2">{k.time}</td>
                          <td className="text-right p-2">{formatCurrency(k.open)}</td>
                          <td className="text-right p-2 text-red-500">{formatCurrency(k.high)}</td>
                          <td className="text-right p-2 text-green-500">{formatCurrency(k.low)}</td>
                          <td className="text-right p-2">{formatCurrency(k.close)}</td>
                          <td className="text-right p-2">{formatVolume(k.volume)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </div>
  );
}
