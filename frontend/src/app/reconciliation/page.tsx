'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { formatCurrency, formatNumber } from '@/lib/utils';
import { Wallet, RefreshCw, AlertCircle, CheckCircle, Clock, XCircle } from 'lucide-react';

interface ReconciliationReport {
  report_id: string;
  check_time: string;
  check_type: string;
  total_users: number;
  issue_users: number;
  critical_count: number;
  high_count: number;
  medium_count: number;
  low_count: number;
  auto_repaired: number;
  duration_ms: number;
}

interface ReconciliationIssue {
  issue_id: string;
  report_id: string;
  user_id: number | null;
  symbol: string;
  category: string;
  severity: string;
  description: string;
  expected_value: number;
  actual_value: number;
  difference: number;
  status: string;
  created_at: string;
}

export default function ReconciliationPage() {
  const router = useRouter();
  const [reports, setReports] = useState<ReconciliationReport[]>([]);
  const [issues, setIssues] = useState<ReconciliationIssue[]>([]);
  const [selectedReport, setSelectedReport] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem('token');
    if (!token) {
      router.push('/');
      return;
    }

    fetchReports();
  }, [router]);

  const getAuthHeaders = () => ({
    'Authorization': `Bearer ${localStorage.getItem('token')}`,
    'Content-Type': 'application/json',
  });

  const fetchReports = async () => {
    setIsLoading(true);
    try {
      const endDate = new Date().toISOString().split('T')[0];
      const startDate = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
      
      const response = await fetch(`http://localhost:8080/api/v1/reconciliation?start_date=${startDate}&end_date=${endDate}`, {
        headers: getAuthHeaders(),
      });
      
      if (response.ok) {
        const data = await response.json();
        setReports(data.reports || []);
      }
    } catch (error) {
      console.error('获取对账报告失败:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const fetchIssues = async (reportId: string) => {
    setIsLoading(true);
    try {
      const response = await fetch(`http://localhost:8080/api/v1/reconciliation/issues?report_id=${reportId}`, {
        headers: getAuthHeaders(),
      });
      
      if (response.ok) {
        const data = await response.json();
        setIssues(data.issues || []);
      }
    } catch (error) {
      console.error('获取对账差异失败:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleReportSelect = (reportId: string) => {
    setSelectedReport(reportId);
    fetchIssues(reportId);
  };

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'CRITICAL': return 'text-red-600 bg-red-100';
      case 'HIGH': return 'text-orange-600 bg-orange-100';
      case 'MEDIUM': return 'text-yellow-600 bg-yellow-100';
      case 'LOW': return 'text-blue-600 bg-blue-100';
      default: return 'text-gray-600 bg-gray-100';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'PENDING': return <Clock className="h-4 w-4 text-yellow-500" />;
      case 'AUTO_REPAIRED': return <CheckCircle className="h-4 w-4 text-green-500" />;
      case 'MANUAL_REPAIRED': return <CheckCircle className="h-4 w-4 text-blue-500" />;
      case 'IGNORED': return <XCircle className="h-4 w-4 text-gray-500" />;
      default: return <Clock className="h-4 w-4 text-gray-500" />;
    }
  };

  const getCategoryName = (category: string) => {
    const names: Record<string, string> = {
      'ASSET_IMBALANCE': '资产不平衡',
      'CASH_MISMATCH': '现金不匹配',
      'FEE_MISMATCH': '手续费不匹配',
      'NEGATIVE_BALANCE': '余额为负',
      'QUANTITY_MISMATCH': '数量不匹配',
      'COST_MISMATCH': '成本价不匹配',
      'NEGATIVE_AVAILABLE': '可用为负',
      'OVER_TRADE': '超额成交',
      'ORPHAN_TRADE': '无主成交',
      'INVALID_PRICE': '无效价格',
      'INVALID_QUANTITY': '无效数量',
      'AMOUNT_MISMATCH': '金额不匹配',
      'TIME_DISORDER': '时间乱序',
      'STATUS_ERROR': '状态错误',
    };
    return names[category] || category;
  };

  const getCheckTypeName = (type: string) => {
    const names: Record<string, string> = {
      'FUND': '资金对账',
      'POSITION': '持仓对账',
      'TRADE': '成交对账',
      'ORDER': '订单对账',
      'FULL': '全量对账',
      'DEEP': '深度对账',
    };
    return names[type] || type;
  };

  const stats = {
    total: reports.length,
    critical: reports.reduce((sum, r) => sum + r.critical_count, 0),
    high: reports.reduce((sum, r) => sum + r.high_count, 0),
    medium: reports.reduce((sum, r) => sum + r.medium_count, 0),
    low: reports.reduce((sum, r) => sum + r.low_count, 0),
    autoRepaired: reports.reduce((sum, r) => sum + r.auto_repaired, 0),
  };

  return (
    <div className="min-h-screen bg-gray-100">
      <nav className="bg-blue-600 text-white p-4">
        <div className="max-w-7xl mx-auto flex justify-between items-center">
          <div className="flex items-center gap-2">
            <Wallet className="h-6 w-6" />
            <span className="text-xl font-bold">对账系统</span>
          </div>
          <div className="flex items-center gap-4">
            <Button variant="secondary" size="sm" onClick={() => router.push('/trade')}>
              交易
            </Button>
            <Button variant="secondary" size="sm" onClick={() => router.push('/market')}>
              行情
            </Button>
            <Button variant="secondary" size="sm" onClick={() => router.push('/account')}>
              账户
            </Button>
          </div>
        </div>
      </nav>

      <div className="max-w-7xl mx-auto p-4">
        <div className="grid grid-cols-6 gap-4 mb-4">
          <Card>
            <CardContent className="pt-6">
              <div className="text-sm text-gray-500">对账次数</div>
              <div className="text-2xl font-bold">{stats.total}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-sm text-gray-500 text-red-500">严重问题</div>
              <div className="text-2xl font-bold text-red-500">{stats.critical}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-sm text-gray-500 text-orange-500">高级问题</div>
              <div className="text-2xl font-bold text-orange-500">{stats.high}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-sm text-gray-500 text-yellow-500">中级问题</div>
              <div className="text-2xl font-bold text-yellow-500">{stats.medium}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-sm text-gray-500 text-blue-500">低级问题</div>
              <div className="text-2xl font-bold text-blue-500">{stats.low}</div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-sm text-gray-500">自动修复</div>
              <div className="text-2xl font-bold text-green-500">{stats.autoRepaired}</div>
            </CardContent>
          </Card>
        </div>

        <div className="grid grid-cols-3 gap-4">
          <Card className="col-span-1">
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-lg">对账报告</CardTitle>
              <Button variant="outline" size="sm" onClick={fetchReports}>
                <RefreshCw className="h-4 w-4" />
              </Button>
            </CardHeader>
            <CardContent>
              <div className="space-y-2 max-h-[600px] overflow-y-auto">
                {reports.map((report) => (
                  <div
                    key={report.report_id}
                    onClick={() => handleReportSelect(report.report_id)}
                    className={`p-3 rounded cursor-pointer ${
                      selectedReport === report.report_id ? 'bg-blue-100' : 'hover:bg-gray-100'
                    }`}
                  >
                    <div className="flex justify-between items-center">
                      <span className="font-medium">{getCheckTypeName(report.check_type)}</span>
                      <span className="text-xs text-gray-500">
                        {new Date(report.check_time).toLocaleDateString()}
                      </span>
                    </div>
                    <div className="flex gap-2 mt-1 text-xs">
                      {report.critical_count > 0 && (
                        <span className="text-red-500">{report.critical_count}严重</span>
                      )}
                      {report.high_count > 0 && (
                        <span className="text-orange-500">{report.high_count}高级</span>
                      )}
                      {report.issue_users > 0 && (
                        <span className="text-gray-500">{report.issue_users}用户</span>
                      )}
                    </div>
                  </div>
                ))}
                {reports.length === 0 && (
                  <div className="text-center py-4 text-gray-500">
                    暂无对账记录
                  </div>
                )}
              </div>
            </CardContent>
          </Card>

          <Card className="col-span-2">
            <CardHeader>
              <CardTitle className="flex justify-between items-center">
                <span>对账差异详情</span>
                {selectedReport && (
                  <span className="text-sm text-gray-500">
                    报告ID: {selectedReport.slice(0, 8)}
                  </span>
                )}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <div className="text-center py-4 text-gray-500">
                  加载中...
                </div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b">
                        <th className="text-left p-2">问题ID</th>
                        <th className="text-left p-2">类别</th>
                        <th className="text-center p-2">级别</th>
                        <th className="text-left p-2">描述</th>
                        <th className="text-right p-2">差异</th>
                        <th className="text-center p-2">状态</th>
                        <th className="text-center p-2">时间</th>
                      </tr>
                    </thead>
                    <tbody>
                      {issues.map((issue) => (
                        <tr key={issue.issue_id} className="border-b hover:bg-gray-50">
                          <td className="p-2 text-xs text-gray-500">{issue.issue_id.slice(0, 8)}</td>
                          <td className="p-2">{getCategoryName(issue.category)}</td>
                          <td className="text-center p-2">
                            <span className={`px-2 py-1 rounded text-xs ${getSeverityColor(issue.severity)}`}>
                              {issue.severity}
                            </span>
                          </td>
                          <td className="p-2 max-w-xs truncate" title={issue.description}>
                            {issue.description}
                          </td>
                          <td className="text-right p-2">
                            {issue.difference !== 0 ? formatCurrency(issue.difference) : '-'}
                          </td>
                          <td className="text-center p-2">
                            <div className="flex items-center justify-center gap-1">
                              {getStatusIcon(issue.status)}
                              <span className="text-xs">
                                {issue.status === 'PENDING' ? '待处理' :
                                 issue.status === 'AUTO_REPAIRED' ? '已自动修复' :
                                 issue.status === 'MANUAL_REPAIRED' ? '已人工修复' : '已忽略'}
                              </span>
                            </div>
                          </td>
                          <td className="text-center p-2 text-xs text-gray-500">
                            {new Date(issue.created_at).toLocaleString()}
                          </td>
                        </tr>
                      ))}
                      {issues.length === 0 && (
                        <tr>
                          <td colSpan={7} className="text-center p-4 text-gray-500">
                            {selectedReport ? '该报告暂无差异' : '请选择报告查看差异'}
                          </td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
