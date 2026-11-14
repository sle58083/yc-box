// 全局变量类型声明：面板访问路径由部署脚本/nginx 在 index.html 中注入。
interface Window {
  /**
   * 面板访问路径，如 '/a1b2c3'；空字符串或 undefined 表示根路径（兼容旧实例）。
   * 前端用其作为 react-router 的 basename。
   */
  __PANEL_PATH__?: string
}
