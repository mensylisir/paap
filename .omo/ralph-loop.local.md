---
active: true
iteration: 1
max_iterations: 500
completion_promise: "DONE"
initial_completion_promise: "DONE"
started_at: "2026-06-23T09:59:24.285Z"
session_id: "ses_10c7c482fffe03Oh38NK5Zjs2X"
ultrawork: true
strategy: "continue"
message_count_at_start: 662
---
1. 选取一个最简单的最容易实现的功能，注意界面样式和风格要一致，要美观，要用户体验好，拿不准的使用mcp上网搜索同类产品是怎么设计和展示的。你严格遵守DESIGN.md和frontend/src/styles/carbon-theme.css定义的token和并正确使用，项目的风格和样式需要一一致，统一使用white主题
2. 实现完成后构建和部署，注意tag号加一，注意检查kind集群的各种资源状态，kind集群无法pull镜像，需要主机pull好导入
3. 使用cdp操作chrome进行测试，注意测试要详细,打开chrome时复用用户的配置，别新整配置目录
4. 测试完成后标记task.md中的相应的任务为完成
5. 提交代码
6. 继续从第一个步骤开始
