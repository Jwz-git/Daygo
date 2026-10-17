export default {
  archetype: {
    label: '元类型',
    hint: '用于决定该分类在 3D 世界中的建筑与自然景观形态。',
    forge: '创造',
    sanctum: '思考',
    nexus: '连接',
    camp: '休憩',
    order: '秩序',
  },
  profession: {
    softwareEngineer: {
      name: '软件工程师',
      categories: {
        coding: { name: '编程 / 调试', details: '在 IDE 或终端里编写、重构和修复代码' },
        codeReview: { name: '代码评审', details: '评审 PR、阅读 diff 和留下评论' },
        research: { name: '研究', details: '阅读文档和 Stack Overflow、了解工具与 API，以及撰写设计文档或技术方案' },
        communication: { name: '沟通', details: '会议、站会、Slack、邮件、视频通话、即时消息和同步沟通' },
      },
    },
    founderExecutive: {
      name: '创始人 / 管理者',
      categories: {
        building: { name: '工程 / 产品', details: '写代码、做设计、发布功能等亲自动手的构建工作' },
        strategy: { name: '研究与战略', details: '竞品研究、定位、深度思考和融资准备' },
        insights: { name: '数据与洞察', details: '数据看板、留存数据、转化漏斗和财务' },
        communication: { name: '沟通', details: '团队同步、投资人通话、用户演示和招聘' },
      },
    },
    designer: {
      name: '设计师',
      categories: {
        design: { name: '设计', details: '原型、UI 组件、用户流程、视觉设计和交付标注' },
        research: { name: '研究', details: '浏览设计模式、竞品审查、用户研究和查看指标' },
        communication: { name: '沟通', details: '设计评审、站会、评图会和方案展示' },
      },
    },
    student: {
      name: '学生',
      categories: {
        studying: { name: '学习', details: '听课、阅读、复习课件、记忆卡片和课程资料' },
        assignments: { name: '作业', details: '论文、习题、编程项目和实验报告' },
        communication: { name: '沟通', details: '学习小组、答疑时间、群聊和给老师发邮件' },
      },
    },
    productManager: {
      name: '产品经理',
      categories: {
        specs: { name: '需求与规划', details: 'PRD、路线图、需求池梳理、迭代规划和工单' },
        research: { name: '研究与分析', details: '用户研究、指标复盘、竞品分析和 A/B 测试' },
        communication: { name: '沟通', details: '站会、干系人同步、设计评审和与研发对齐' },
      },
    },
    dataScientist: {
      name: '数据科学家',
      categories: {
        analysis: { name: '分析与建模', details: 'Notebook、统计分析、机器学习训练和数据探索' },
        dataEngineering: { name: '数据工程', details: 'SQL 查询、数据管道、数据清洗和 ETL 脚本' },
        research: { name: '研究', details: '阅读论文和文档，探索新的方法与工具' },
        communication: { name: '沟通', details: '汇报结论、干系人同步和团队讨论' },
      },
    },
    other: {
      name: '其他',
      categories: {
        work: { name: '工作', details: '无法归入更具体类别的专注工作和职责事务' },
        communication: { name: '沟通', details: '会议、站会、Slack、邮件、视频通话、即时消息和同步沟通' },
      },
    },
  },
  common: {
    distraction: { name: '分心', details: '无目的的浏览与被动消费内容：刷社交媒体、随便看视频、无意义地滚动、没有明确意图的娱乐，以及打游戏' },
    personal: { name: '个人', details: '有明确目的的非工作活动：联系亲友、处理财务、订行程、办事、生活琐事和爱好' },
  },
}
