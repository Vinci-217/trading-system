'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { AlertCircle, TrendingUp } from 'lucide-react';

interface LoginResponse {
  token: string;
  user_id: number;
  username: string;
}

export default function Home() {
  const router = useRouter();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');

  async function onLogin(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsLoading(true);
    setError('');

    const formData = new FormData(event.currentTarget);
    const username = formData.get('username');
    const password = formData.get('password');

    try {
      const response = await fetch('http://localhost:8080/api/v1/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ username, password }),
      });

      const data: LoginResponse = await response.json();

      if (!response.ok) {
        throw new Error(data.token || '登录失败');
      }

      localStorage.setItem('token', data.token);
      localStorage.setItem('user_id', String(data.user_id));
      localStorage.setItem('username', data.username);

      router.push('/trade');
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败');
    } finally {
      setIsLoading(false);
    }
  }

  async function onRegister(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsLoading(true);
    setError('');

    const formData = new FormData(event.currentTarget);
    const username = formData.get('username');
    const password = formData.get('password');
    const email = formData.get('email');

    try {
      const response = await fetch('http://localhost:8080/api/v1/register', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ username, password, email }),
      });

      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error || '注册失败');
      }

      setError('');
      alert('注册成功，请登录');
    } catch (err) {
      setError(err instanceof Error ? err.message : '注册失败');
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-blue-50 to-indigo-100">
      <div className="w-full max-w-md px-4">
        <div className="flex items-center justify-center mb-8">
          <TrendingUp className="h-10 w-10 text-blue-600 mr-3" />
          <h1 className="text-3xl font-bold text-gray-900">证券交易系统</h1>
        </div>

        <Card>
          <CardHeader className="text-center">
            <CardTitle>欢迎登录</CardTitle>
            <CardDescription>
              请登录您的账户或注册新账户
            </CardDescription>
          </CardHeader>
          <CardContent>
            {error && (
              <div className="flex items-center gap-2 p-3 mb-4 text-sm text-red-600 bg-red-50 rounded-md">
                <AlertCircle className="h-4 w-4" />
                {error}
              </div>
            )}

            <Tabs defaultValue="login" className="w-full">
              <TabsList className="grid w-full grid-cols-2">
                <TabsTrigger value="login">登录</TabsTrigger>
                <TabsTrigger value="register">注册</TabsTrigger>
              </TabsList>

              <TabsContent value="login">
                <form onSubmit={onLogin} className="space-y-4">
                  <div className="space-y-2">
                    <Label htmlFor="login-username">用户名</Label>
                    <Input
                      id="login-username"
                      name="username"
                      placeholder="请输入用户名"
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="login-password">密码</Label>
                    <Input
                      id="login-password"
                      name="password"
                      type="password"
                      placeholder="请输入密码"
                      required
                    />
                  </div>
                  <Button type="submit" className="w-full" disabled={isLoading}>
                    {isLoading ? '登录中...' : '登录'}
                  </Button>
                </form>
              </TabsContent>

              <TabsContent value="register">
                <form onSubmit={onRegister} className="space-y-4">
                  <div className="space-y-2">
                    <Label htmlFor="register-username">用户名</Label>
                    <Input
                      id="register-username"
                      name="username"
                      placeholder="请输入用户名（3-50字符）"
                      minLength={3}
                      maxLength={50}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="register-email">邮箱</Label>
                    <Input
                      id="register-email"
                      name="email"
                      type="email"
                      placeholder="请输入邮箱（可选）"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="register-password">密码</Label>
                    <Input
                      id="register-password"
                      name="password"
                      type="password"
                      placeholder="请输入密码（至少6字符）"
                      minLength={6}
                      required
                    />
                  </div>
                  <Button type="submit" className="w-full" disabled={isLoading}>
                    {isLoading ? '注册中...' : '注册'}
                  </Button>
                </form>
              </TabsContent>
            </Tabs>
          </CardContent>
        </Card>

        <p className="text-center text-sm text-gray-500 mt-4">
          注册即获100,000模拟资金
        </p>
      </div>
    </div>
  );
}
