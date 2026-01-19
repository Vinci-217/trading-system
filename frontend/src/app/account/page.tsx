'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { formatCurrency, formatNumber, formatPercentage } from '@/lib/utils';
import { Wallet, TrendingUp, TrendingDown, BarChart, FileText, RefreshCw } from 'lucide-react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts';

interface Account {
  user_id: number;
  cash_balance: number;
  frozen_balance: number;
  total_assets: number;
  total_profit: number;
}

interface Position {
  symbol: string;
  symbol_name: string;
  quantity: number;
  available_quantity: number;
  cost_price: number;
  current_price: number;
  market_value: number;
  profit_loss: number;
  profit_loss_rate: number;
}

interface Trade {
  trade_id: string;
  symbol: string;
  side: string;
  price: number;
  quantity: number;
  amount: number;
  profit_loss: number;
  created_at: string;
}

export default function AccountPage() {
  const router = useRouter();
  const [account, setAccount] = useState<Account | null>(null);
  const [positions, setPositions] = useState<Position[]>([]);
  const [trades, setTrades] = useState<Trade[]>([]);
  const [activeTab, setActiveTab] = useState('assets');

  useEffect(() => {
    const token = localStorage.getItem('token');
    if (!token) {
      router.push('/');
      return;
    }

    fetchData();
  }, [router]);

  const getAuthHeaders = () => ({
    'Authorization': `Bearer ${localStorage.getItem('token')}`,
    'Content-Type': 'application/json',
  });

  const fetchData = async () => {
    await Promise.all([
      fetchAccount(),
      fetchPositions(),
      fetchTrades(),
    ]);
  };

  const fetchAccount = async () => {
    try {
      const response = await fetch('http://localhost:8080/api/v1/account', {
        headers: getAuthHeaders(),
      });
      if (response.ok) {
        const data = await response.json();
        setAccount(data);
      }
    } catch (error) {
      console.error('获取账户失败:', error);
    }
  };

  const fetchPositions = async () => {
    try {
      const response = await fetch('http://localhost:8080/api/v1/positions', {
        headers: getAuthHeaders(),
      });
      if (response.ok) {
        const data = await response.json();
        setPositions(data.positions || []);
      }
    } catch (error) {
      console.error('获取持仓失败:', error);
    }
  };

  const fetchTrades = async () => {
    try {
      const response = await fetch('http://localhost:8080/api/v1/trades?limit=50', {
        headers: getAuthHeaders(),
      });
      if (response.ok) {
        const data = await response.json();
        setTrades(data.trades || []);
      }
    } catch (error) {
      console.error('获取成交记录失败:', error);
    }
  };

  const assetAllocation = positions.map(p => ({
    name: p.symbol_name,
    value: p.market_value,
  }));

  const COLORS = ['#2563eb', '#7c3aed', '#db2777', '#ea580c', '#16a34a', '#0891b2'];

  const chartData = positions.map(p => ({
    name: p.symbol,
    value: p.market_value,
    profit: p.profit_loss,
  }));

  return (
    <div className="min-h-screen bg-gray-100">
      <nav className="bg-blue-600 text-white p-4">
        <div className="max-w-7xl mx-auto flex justify-between items-center">
          <div className="flex items-center gap-2">
            <Wallet className="h-6 w-6" />
            <span className="text-xl font-bold">账户中心</span>
          </div>
          <div className="flex items-center gap-4">
            <Button variant="secondary" size="sm" onClick={() => router.push('/trade')}>
              交易
            </Button>
            <Button variant="secondary" size="sm" onClick={() => router.push('/market')}>
              行情
            </Button>
          </div>
        </div>
      </nav>

      <div className="max-w-7xl mx-auto p-4">
        <div className="grid grid-cols-4 gap-4 mb-4">
          <Card>
            <CardContent className="pt-6">
              <div className="text-sm text-gray-500">可用资金</div>
              <div className="text-2xl font-bold">{formatCurrency(account?.cash_balance || 0)}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-sm text-gray-500">冻结资金</div>
              <div className="text-2xl font-bold">{formatCurrency(account?.frozen_balance || 0)}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-sm text-gray-500">总资产</div>
              <div className="text-2xl font-bold">{formatCurrency(account?.total_assets || 0)}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-sm text-gray-500">总盈亏</div>
              <div className={`text-2xl font-bold ${(account?.total_profit || 0) >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                {(account?.total_profit || 0) >= 0 ? '+' : ''}{formatCurrency(account?.total_profit || 0)}
              </div>
            </CardContent>
          </Card>
        </div>

        <div className="grid grid-cols-3 gap-4">
          <Card className="col-span-2">
            <CardHeader>
              <CardTitle className="flex justify-between items-center">
                <span>资产分布</span>
                <Button variant="outline" size="sm" onClick={fetchPositions}>
                  <RefreshCw className="h-4 w-4" />
                </Button>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="h-[300px]">
                {positions.length > 0 ? (
                  <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                      <Pie
                        data={chartData}
                        cx="50%"
                        cy="50%"
                        innerRadius={60}
                        outerRadius={100}
                        paddingAngle={5}
                        dataKey="value"
                        label={({ name, percent }) => `${name} ${(percent * 100).toFixed(1)}%`}
                      >
                        {chartData.map((entry, index) => (
                          <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                        ))}
                      </Pie>
                      <Tooltip formatter={(value: number) => formatCurrency(value)} />
                    </PieChart>
                  </ResponsiveContainer>
                ) : (
                  <div className="flex items-center justify-center h-full text-gray-500">
                    暂无持仓数据
                  </div>
                )}
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>持仓概况</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-3">
                <div className="flex justify-between">
                  <span className="text-gray-500">持仓股票数</span>
                  <span className="font-medium">{positions.length}只</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">持仓市值</span>
                  <span className="font-medium">
                    {formatCurrency(positions.reduce((sum, p) => sum + p.market_value, 0))}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">持仓盈亏</span>
                  <span className={`font-medium ${positions.reduce((sum, p) => sum + p.profit_loss, 0) >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                    {formatCurrency(positions.reduce((sum, p) => sum + p.profit_loss, 0))}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">平均持仓成本</span>
                  <span className="font-medium">
                    {positions.length > 0 
                      ? formatCurrency(positions.reduce((sum, p) => sum + p.cost_price * p.quantity, 0) / positions.reduce((sum, p) => sum + p.quantity, 0))
                      : '0.00'
                    }
                  </span>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card className="col-span-3">
            <CardHeader>
              <CardTitle className="flex justify-between items-center">
                <span>持仓明细</span>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b">
                      <th className="text-left p-2">股票</th>
                      <th className="text-right p-2">持仓</th>
                      <th className="text-right p-2">可用</th>
                      <th className="text-right p-2">成本价</th>
                      <th className="text-right p-2">现价</th>
                      <th className="text-right p-2">市值</th>
                      <th className="text-right p-2">盈亏</th>
                      <th className="text-right p-2">收益率</th>
                    </tr>
                  </thead>
                  <tbody>
                    {positions.map((pos) => (
                      <tr key={pos.symbol} className="border-b hover:bg-gray-50">
                        <td className="p-2">
                          <div className="font-medium">{pos.symbol_name}</div>
                          <div className="text-gray-500 text-xs">{pos.symbol}</div>
                        </td>
                        <td className="text-right p-2">{pos.quantity}</td>
                        <td className="text-right p-2">{pos.available_quantity}</td>
                        <td className="text-right p-2">{formatCurrency(pos.cost_price)}</td>
                        <td className="text-right p-2">{formatCurrency(pos.current_price)}</td>
                        <td className="text-right p-2">{formatCurrency(pos.market_value)}</td>
                        <td className={`text-right p-2 ${pos.profit_loss >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                          {formatCurrency(pos.profit_loss)}
                        </td>
                        <td className={`text-right p-2 ${pos.profit_loss_rate >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                          {formatPercentage(pos.profit_loss_rate)}
                        </td>
                      </tr>
                    ))}
                    {positions.length === 0 && (
                      <tr>
                        <td colSpan={8} className="text-center p-4 text-gray-500">
                          暂无持仓
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>

          <Card className="col-span-3">
            <CardHeader>
              <CardTitle className="flex justify-between items-center">
                <span>成交记录</span>
                <Button variant="outline" size="sm" onClick={fetchTrades}>
                  <RefreshCw className="h-4 w-4" />
                </Button>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b">
                      <th className="text-left p-2">成交号</th>
                      <th className="text-left p-2">股票</th>
                      <th className="text-center p-2">方向</th>
                      <th className="text-right p-2">价格</th>
                      <th className="text-right p-2">数量</th>
                      <th className="text-right p-2">金额</th>
                      <th className="text-right p-2">盈亏</th>
                      <th className="text-center p-2">时间</th>
                    </tr>
                  </thead>
                  <tbody>
                    {trades.map((trade) => (
                      <tr key={trade.trade_id} className="border-b hover:bg-gray-50">
                        <td className="p-2 text-xs text-gray-500">{trade.trade_id.slice(0, 8)}</td>
                        <td className="p-2">{trade.symbol}</td>
                        <td className={`text-center p-2 ${trade.side === 'BUY' ? 'text-red-500' : 'text-green-500'}`}>
                          {trade.side === 'BUY' ? '买入' : '卖出'}
                        </td>
                        <td className="text-right p-2">{formatCurrency(trade.price)}</td>
                        <td className="text-right p-2">{trade.quantity}</td>
                        <td className="text-right p-2">{formatCurrency(trade.amount)}</td>
                        <td className={`text-right p-2 ${trade.profit_loss >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                          {trade.profit_loss ? formatCurrency(trade.profit_loss) : '-'}
                        </td>
                        <td className="text-center p-2 text-xs text-gray-500">
                          {new Date(trade.created_at).toLocaleString()}
                        </td>
                      </tr>
                    ))}
                    {trades.length === 0 && (
                      <tr>
                        <td colSpan={8} className="text-center p-4 text-gray-500">
                          暂无成交记录
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
