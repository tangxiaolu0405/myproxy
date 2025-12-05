``` Mermaid
sequenceDiagram
    participant User as 用户
    participant Client as SOCKS5 客户端 (你的 Go 程序)
    participant Proxy as SOCKS5 代理服务器 (你的 Go Server)
    participant Target as 目标服务器 (如 example.com:80)

    User->>Client: 想要访问 Target
    
    rect rgb(200, 255, 200)
        Client->>Proxy: 建立 TCP 连接
        Note over Client, Proxy: 阶段 1: 协商 (Negotiation)
        Client->>Proxy: 发送 SOCKS5 版本及支持的认证方法 (0x05, NMETHODS, METHODS)
        Proxy->>Client: 回复选择的认证方法 (0x05, METHOD_SELECTED)
    end
    
    opt 认证阶段 (Auth)
        Note over Client, Proxy: 阶段 2: 认证 (如果需要)
        Client->>Proxy: 发送用户名/密码 (0x01, ULEN, UNAME, PLEN, PASSWD)
        Proxy->>Client: 回复认证状态 (0x01, STATUS)
    end
    
    rect rgb(255, 255, 200)
        Note over Client, Proxy: 阶段 3: 请求 (Request)
        Client->>Proxy: 发送 CONNECT 请求 (0x05, 0x01, 目标地址, 目标端口)
        
        Proxy->>Target: 建立 TCP 连接到 目标服务器
        
        Note over Client, Proxy: 阶段 4: 响应 (Reply)
        Proxy->>Client: 回复连接成功 (0x05, 0x00, 绑定地址, 绑定端口)
    end
    
    Client-->>User: 连接建立成功！

    loop 数据转发
        Client->>Proxy: 发送数据给 Target
        Proxy->>Target: 转发数据
        
        Target->>Proxy: 回复数据给 Client
        Proxy->>Client: 转发数据
    end
    
    Client->>Proxy: 关闭连接
    Proxy->>Target: 关闭连接

```