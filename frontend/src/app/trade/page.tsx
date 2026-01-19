'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { formatCurrency, formatNumber, formatPercentage } from '@/lib/utils';
import { TrendingUp, TrendingDown, RefreshCw, Download, Wallet, FileText, BarChart } from 'lucide-react';

interface Quote {
  symbol: string;
  symbol_name: string;
  price: number;
  change: number;
  change_percent: number;
  high: number;
  low: number;
  volume: number;
}

interface Order {
  order_id: string;
  symbol: string;
  symbol_name: string;
  order_type: string;
  side: string;
  price: number;
  quantity: number;
  filled_quantity: number;
  status: string;
  created_at: string;
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

interface Account {
  user_id: number;
  cash_balance: number;
  frozen_balance: number;
  total_assets: number;
  total_profit: number;
}

export default function TradePage() {
  const router = useRouter();
  const [selectedSymbol, setSelectedSymbol] = useState('600519');
  const [quote, setQuote] = useState<Quote | null>(null);
  const [orders, setOrders] = useState<Order[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [account, setAccount] = useState<Account | null>(null);
  const [orderType, setOrderType] = useState('LIMIT');
  const [orderSide, setOrderSide] = useState('BUY');
  const [orderPrice, setOrderPrice] = useState('');
  const [orderQuantity, setOrderQuantity] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem('token');
    if (!token) {
      router.push('/');
      return;
    }

    fetchData();
    const interval = setInterval(fetchQuote, 3000);
    return () => clearInterval(interval);
  }, [router, selectedSymbol]);

  const getAuthHeaders = () => ({
    'Authorization': `Bearer ${localStorage.getItem('token')}`,
    'Content-Type': 'application/json',
  });

  const fetchData = async () => {
    await Promise.all([
      fetchQuote(),
      fetchOrders(),
      fetchPositions(),
      fetchAccount(),
    ]);
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

  const fetchOrders = async () => {
    try {
      const response = await fetch('http://localhost:8080/api/v1/orders?limit=50', {
        headers: getAuthHeaders(),
      });
      if (response.ok) {
        const data = await response.json();
        setOrders(data.orders || []);
      }
    } catch (error) {
      console.error('获取订单失败:', error);
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

  const submitOrder = async () => {
    if (!orderPrice || !orderQuantity) {
      alert('请填写价格和数量');
      return;
    }

    setIsLoading(true);
    try {
      const response = await fetch('http://localhost:8080/api/v1/orders', {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify({
          symbol: selectedSymbol,
          order_type: orderType,
          side: orderSide,
          price: parseFloat(orderPrice),
          quantity: parseInt(orderQuantity),
        }),
      });

      if (response.ok) {
        alert('下单成功');
        setOrderPrice('');
        setOrderQuantity('');
        fetchData();
      } else {
        const data = await response.json();
        alert(data.error || '下单失败');
      }
    } catch (error) {
      alert('下单失败');
    } finally {
      setIsLoading(false);
    }
  };

  const cancelOrder = async (orderId: string) => {
    if (!confirm('确定要撤单吗？')) return;

    try {
      const response = await fetch(`http://localhost:8080/api/v1/orders/${orderId}`, {
        method: 'DELETE',
        headers: getAuthHeaders(),
      });

      if (response.ok) {
        alert('撤单成功');
        fetchOrders();
      } else {
        alert('撤单失败');
      }
    } catch (error) {
      alert('撤单失败');
    }
  };

  const exportOrders = async () => {
    try {
      const response = await fetch('http://localhost:8080/api/v1/export/orders', {
        method: 'POST',
        headers: getAuthHeaders(),
      });

      if (response.ok) {
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `orders_${new Date().toISOString().split('T')[0]}.csv`;
        a.click();
      }
    } catch (error) {
      alert('导出失败');
    }
  };

  const symbols = ['600519', '000001', '600036', '000002', '600276', '000651', '600030', '000725', '600887', '002594'];

  return (
    <div className="min-h-screen bg-gray-100">
      <nav className="bg-blue-600 text-white p-4">
        <div className="max-w-7xl mx-auto flex justify-between items-center">
          <div className="flex items-center gap-2">
            <TrendingUp className="h-6 w-6" />
            <span className="text-xl font-bold">证券交易系统</span>
          </div>
          <div className="flex items-center gap-4">
            <span>用户: {localStorage.getItem('username')}</span>
            <Button variant="secondary" size="sm" onClick={() => router.push('/account')}>
              <Wallet className="h-4 w-4 mr-2" />
              账户
            </Button>
            <Button variant="secondary" size="sm" onClick={() => router.push('/market')}>
              <BarChart className="h-4 w-4 mr-2" />
              行情
            </Button>
          </div>
        </div>
      </nav>

      <div className="max-w-7xl mx-auto p-4">
        <div className="grid grid-cols-4 gap-4">
          <Card className="col-span-1">
            <CardHeader>
              <CardTitle className="text-lg">股票选择</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-2">
                {symbols.map(symbol => (
                  <button
                    key={symbol}
                    onClick={() => setSelectedSymbol(symbol)}
                    className={`w-full p-2 text-left rounded ${
                      selectedSymbol === symbol ? 'bg-blue-100' : 'hover:bg-gray-100'
                    }`}
                  >
                    {symbol}
                  </button>
                ))}
              </div>
            </CardContent>
          </Card>

          <Card className="col-span-2">
            <CardHeader>
              <CardTitle className="flex justify-between items-center">
                <span>{quote?.symbol_name || selectedSymbol} ({selectedSymbol})</span>
                <span className={`text-2xl ${quote?.change >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                  {formatCurrency(quote?.price || 0)}
                </span>
              </CardTitle>
            </CardHeader>
            <CardContent>
              {quote && (
                <div className="grid grid-cols-4 gap-4 text-sm">
                  <div>
                    <span className="text-gray-500">涨跌</span>
                    <p className={quote.change >= 0 ? 'text-red-500' : 'text-green-500'}>
                      {quote.change >= 0 ? '+' : ''}{formatCurrency(quote.change)}
                    </p>
                  </div>
                  <div>
                    <span className="text-gray-500">涨跌幅</span>
                    <p className={quote.change_percent >= 0 ? 'text-red-500' : 'text-green-500'}>
                      {formatPercentage(quote.change_percent)}
                    </p>
                  </div>
                  <div>
                    <span className="text-gray-500">最高</span>
                    <p>{formatCurrency(quote.high)}</p>
                  </div>
                  <div>
                    <span className="text-gray-500">最低</span>
                    <p>{formatCurrency(quote.low)}</p>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          <Card className="col-span-1">
            <CardHeader>
              <CardTitle className="text-lg">账户资产</CardTitle>
            </CardHeader>
            <CardContent>
              {account && (
                <div className="space-y-2 text-sm">
                  <div className="flex justify-between">
                    <span>可用资金</span>
                    <span>{formatCurrency(account.cash_balance)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>冻结资金</span>
                    <span>{formatCurrency(account.frozen_balance)}</span>
                  </div>
                  <div className="flex justify-between font-bold">
                    <span>总资产</span>
                    <span>{formatCurrency(account.total_assets)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>总盈亏</span>
                    <span className={account.total_profit >= 0 ? 'text-red-500' : 'text-green-500'}>
                      {formatCurrency(account.total_profit)}
                    </span>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          <Card className="col-span-2">
            <CardHeader>
              <CardTitle className="text-lg">下单面板</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div className="flex gap-2">
                  <Button
                    variant={orderSide === 'BUY' ? 'destructive' : 'outline'}
                    onClick={() => setOrderSide('BUY')}
                    className="flex-1"
                  >
                    <TrendingUp className="h-4 w-4 mr-2" />
                    买入
                  </Button>
                  <Button
                    variant={orderSide === 'SELL' ? 'default' : 'outline'}
                    onClick={() => setOrderSide('SELL')}
                    className="flex-1"
                  >
                    <TrendingDown className="h-4 w-4 mr-2" />
                    卖出
                  </Button>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label>订单类型</Label>
                    <select
                      value={orderType}
                      onChange={(e) => setOrderType(e.target.value)}
                      className="w-full p-2 border rounded"
                    >
                      <option value="LIMIT">限价单</option>
                      <option value="MARKET">市价单</option>
                    </select>
                  </div>
                  <div className="space-y-2">
                    <Label>价格</Label>
                    <Input
                      type="number"
                      value={orderPrice}
                      onChange={(e) => setOrderPrice(e.target.value)}
                      placeholder={orderType === 'MARKET' ? '市价单无需填写' : ''}
                      disabled={orderType === 'MARKET'}
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label>数量</Label>
                  <Input
                    type="number"
                    value={orderQuantity}
                    onChange={(e) => setOrderQuantity(e.target.value)}
                    placeholder="请输入数量（100的倍数）"
                  />
                </div>

                <Button
                  onClick={submitOrder}
                  disabled={isLoading}
                  className={`w-full ${orderSide === 'BUY' ? 'bg-red-500 hover:bg-red-600' : 'bg-green-500 hover:bg-green-600'}`}
                >
                  {isLoading ? '处理中...' : `${orderSide === 'BUY' ? '买入' : '卖出'} ${selectedSymbol}`}
                </Button>
              </div>
            </CardContent>
          </Card>

          <Card className="col-span-2">
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-lg">持仓</CardTitle>
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
                      <th className="text-right p-2">盈亏</th>
                    </tr>
                  </thead>
                  <tbody>
                    {positions.map((pos) => (
                      <tr key={pos.symbol} className="border-b hover:bg-gray-50">
                        <td className="p-2">
                          <div>{pos.symbol_name}</div>
                          <div className="text-gray-500 text-xs">{pos.symbol}</div>
                        </td>
                        <td className="text-right p-2">{pos.quantity}</td>
                        <td className="text-right p-2">{pos.available_quantity}</td>
                        <td className="text-right p-2">{formatCurrency(pos.cost_price)}</td>
                        <td className="text-right p-2">{formatCurrency(pos.current_price)}</td>
                        <td className={`text-right p-2 ${pos.profit_loss >= 0 ? 'text-red-500' : 'text-green-500'}`}>
                          {formatCurrency(pos.profit_loss)}
                        </td>
                      </tr>
                    ))}
                    {positions.length === 0 && (
                      <tr>
                        <td colSpan={6} className="text-center p-4 text-gray-500">
                          暂无持仓
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>

          <Card className="col-span-4">
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-lg">订单列表</CardTitle>
              <Button variant="outline" size="sm" onClick={exportOrders}>
                <Download className="h-4 w-4 mr-2" />
                导出
              </Button>
            </CardHeader>
            <CardContent>
              <Tabs defaultValue="pending">
                <TabsList>
                  <TabsTrigger value="pending">挂单</TabsTrigger>
                  <TabsTrigger value="all">全部</TabsTrigger>
                </TabsList>
                <TabsContent value="pending" className="mt-4">
                  <OrderTable orders={orders.filter(o => ['PENDING', 'PARTIAL'].includes(o.status))} onCancel={cancelOrder} />
                </TabsContent>
                <TabsContent value="all" className="mt-4">
                  <OrderTable orders={orders} onCancel={cancelOrder} />
                </TabsContent>
              </Tabs>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}

function OrderTable({ orders, onCancel }: { orders: Order[]; onCancel: (id: string) => void }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b">
            <th className="text-left p-2">订单号</th>
            <th className="text-left p-2">股票</th>
            <th className="text-center p-2">方向</th>
            <th className="text-center p-2">类型</th>
            <th className="text-right p-2">价格</th>
            <th className="text-right p-2">数量</th>
            <th className="text-right p-2">成交</th>
            <th className="text-center p-2">状态</th>
            <th className="text-center p-2">时间</th>
            <th className="text-center p-2">操作</th>
          </tr>
        </thead>
        <tbody>
          {orders.map((order) => (
            <tr key={order.order_id} className="border-b hover:bg-gray-50">
              <td className="p-2 text-xs text-gray-500">{order.order_id.slice(0, 8)}</td>
              <td className="p-2">
                <div>{order.symbol_name}</div>
                <div className="text-gray-500 text-xs">{order.symbol}</div>
              </td>
              <td className={`text-center p-2 ${order.side === 'BUY' ? 'text-red-500' : 'text-green-500'}`}>
                {order.side === 'BUY' ? '买入' : '卖出'}
              </td>
              <td className="text-center p-2">{order.order_type === 'LIMIT' ? '限价' : '市价'}</td>
              <td className="text-right p-2">{formatCurrency(order.price)}</td>
              <td className="text-right p-2">{order.quantity}</td>
              <td className="text-right p-2">{order.filled_quantity}</td>
              <td className="text-center p-2">
                <span className={`px-2 py-1 rounded text-xs ${
                  order.status === 'FILLED' ? 'bg-green-100 text-green-800' :
                  order.status === 'CANCELLED' ? 'bg-gray-100 text-gray-800' :
                  order.status === 'PARTIAL' ? 'bg-yellow-100 text-yellow-800' :
                  'bg-blue-100 text-blue-800'
                }`}>
                  {order.status === 'PENDING' ? '等待' :
                   order.status === 'PARTIAL' ? '部分成交' :
                   order.status === 'FILLED' ? '全部成交' :
                   order.status === 'CANCELLED' ? '已取消' : '已拒绝'}
                </span>
              </td>
              <td className="text-center p-2 text-xs text-gray-500">
                {new Date(order.created_at).toLocaleTimeString()}
              </td>
              <td className="text-center p-2">
                {['PENDING', 'PARTIAL'].includes(order.status) && (
                  <Button variant="outline" size="sm" onClick={() => onCancel(order.order_id)}>
                    撤单
                  </Button>
                )}
              </td>
            </tr>
          ))}
          {orders.length === 0 && (
            <tr>
              <td colSpan={10} className="text-center p-4 text-gray-500">
                暂无订单
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
