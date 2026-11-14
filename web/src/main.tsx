import React from 'react'
import ReactDOM from 'react-dom/client'
import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import App from './App'
import { getPanelPath } from './lib/panelPath'
import './index.css'

// 读取部署时注入的面板路径（如 '/a1b2c3' 或 ''），作为路由 basename。
const basename = getPanelPath()

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <ConfigProvider locale={zhCN}>
      <App basename={basename} />
    </ConfigProvider>
  </React.StrictMode>,
)
