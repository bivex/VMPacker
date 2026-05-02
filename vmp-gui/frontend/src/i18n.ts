import { createI18n } from 'vue-i18n'

const messages = {
  en: {
    common: {
      workspace: 'Workspace',
      openFile: 'Open Target File',
      startProtection: 'Start Protection',
      projectElements: 'Project Elements',
      targetFunctions: 'Target Functions',
      outputExport: 'Export Settings',
      protectionOptions: 'Protection Options',
      ready: 'Ready',
      coreEngine: 'Core Engine',
      home: 'Home',
      logs: 'Logs',
      help: 'Help / FAQ',
      cancel: 'Cancel',
      confirm: 'Confirm',
      browse: 'Browse...',
      searching: 'Searching...',
      loading: 'Loading sections and symbol table...',
      add: 'Add',
      success: 'Success',
      warning: 'Warning',
      error: 'Error',
      manualAdd: 'Add Manual Function',
      recentFiles: 'Recent Files',
      noRecentFiles: 'No recent files',
      quickActions: 'Quick Actions',
      outputConsole: 'Output Console',
      systemIdle: 'System idle. Ready...',
      protectionComplete: 'Operation complete.',
      operationFailed: 'Operation failed: '
    },
    home: {
      welcomeTitle: 'Virtual Machine Protection System',
      welcomeSubtitle: 'Ready, please open a target program to start.',
      dropZoneText: 'Drop ELF file here, or click to select',
      dropZoneRelease: 'Release to open file',
      supportInfo: 'Supports ARM64 ELF executables',
      viewDocs: 'View Documentation / FAQ',
      recentFilesTitle: 'Recently Opened'
    },
    functions: {
      title: 'Target Functions',
      addFunction: 'Add Function',
      searchPlaceholder: 'Search functions...',
      tableHeaderNode: 'Function Node',
      tableHeaderAddress: 'Address',
      tableHeaderStatus: 'Status',
      statusToBeProtected: 'To Protect',
      statusNotSelected: 'Not Selected',
      noFunctionsFound: 'No protectable objects found in this file.',
      addDialogTitle: 'Add Custom Function',
      methodName: 'Method Name',
      startAddress: 'Start Address',
      endAddress: 'End Address',
      invalidAddress: 'Invalid address format, please enter a valid hexadecimal address.',
      addressError: 'End address must be greater than start address.',
      addSuccess: 'Function {name} added'
    },
    output: {
      title: 'Export Configuration',
      outputPath: 'Save Path (Output Path)',
      outputPathPlaceholder: 'Default saves as _vmp copy in original directory',
      outputPathInfo: 'Protected executable will be saved here.',
      debugPath: 'Debug Mapping File Path',
      debugPathInfo: 'VM instruction mapping will be written here in Debug mode.',
      unsupportedPath: 'Unsupported Instruction Debug File',
      unsupportedPathInfo: 'Detailed info for non-virtualizable instructions will be written here.'
    },
    options: {
      title: 'Protection Options',
      enableDebug: 'Enable Debug Mode',
      enableDebugDesc: 'Output VM execution logs at runtime. Used for analysis or troubleshooting.',
      stripSymbols: 'Strip Symbols',
      stripSymbolsDesc: 'Remove all debug symbols and non-exported names after protection. Recommended for release builds.',
      tokenEntry: 'Tokenized Entry Mode',
      tokenEntryDesc: 'Use compact 3-instruction trampoline instead of default entry to save space.'
    },
    logs: {
      title: 'System Release Notes'
    }
  },
  zh: {
    common: {
      workspace: '工作空间',
      openFile: '打开目标文件',
      startProtection: '开始保护',
      projectElements: '项目元素',
      targetFunctions: '目标函数',
      outputExport: '导出输出',
      protectionOptions: '保护选项',
      ready: '准备就绪',
      coreEngine: 'Core Engine',
      home: '起始页',
      logs: '更新日志',
      help: '查看文档 / 常见问题',
      cancel: '取消',
      confirm: '确认添加',
      browse: '浏览...',
      searching: '搜索函数...',
      loading: '正在解析节区与符号表...',
      add: '添加函数',
      success: '成功',
      warning: '警告',
      error: '错误',
      manualAdd: '添加自定义函数',
      recentFiles: '最近打开文件',
      noRecentFiles: '暂无记录',
      quickActions: '快速操作',
      outputConsole: '输出控制台',
      systemIdle: '系统空闲。准备就绪...',
      protectionComplete: '操作全部完成。',
      operationFailed: '执行失败: '
    },
    home: {
      welcomeTitle: '虚拟机保护系统',
      welcomeSubtitle: '准备就绪，请打开目标程序以开始操作。',
      dropZoneText: '拖拽 ELF 文件到此处，或点击选择',
      dropZoneRelease: '松开以打开文件',
      supportInfo: '支持 ARM64 ELF 可执行文件',
      viewDocs: '查看文档 / 常见问题',
      recentFilesTitle: '最近打开文件'
    },
    functions: {
      title: '目标函数',
      addFunction: '添加函数',
      searchPlaceholder: '搜索函数...',
      tableHeaderNode: '函数节点',
      tableHeaderAddress: '地址',
      tableHeaderStatus: '保护状态',
      statusToBeProtected: '待保护',
      statusNotSelected: '未选择',
      noFunctionsFound: '未在此文件中发现可保护的对象。',
      addDialogTitle: '添加自定义函数',
      methodName: '方法名',
      startAddress: '开始地址',
      endAddress: '结束地址',
      invalidAddress: '地址格式无效，请输入合法的十六进制地址',
      addressError: '结束地址必须大于开始地址',
      addSuccess: '函数 {name} 已添加'
    },
    output: {
      title: '导出输出配置',
      outputPath: '指定保存路径 (Output Path)',
      outputPathPlaceholder: '默认保存到原文件目录下的 _vmp 副本',
      outputPathInfo: '保护打包完成后的可执行文件将保存到此位置。',
      debugPath: 'Debug 对照文件路径',
      debugPathInfo: '开启 Debug 模式后，虚拟机指令对照信息将写入此文件。',
      unsupportedPath: '不支持指令 Debug 文件路径',
      unsupportedPathInfo: '当遇到无法虚拟化的指令时，Packer 会将详细信息写入此文件。'
    },
    options: {
      title: '保护选项设置',
      enableDebug: '开启 Debug 模式',
      enableDebugDesc: '打包后在运行时输出虚拟机调试指令流水线日志。通常用于分析执行流或排除故障。',
      stripSymbols: '去除符号表 (Strip Symbols)',
      stripSymbolsDesc: '保护完成后移除文件中的所有调试符号和未导出函数名，增大逆向分析难度。推荐在发行版中开启。',
      tokenEntry: 'Token 化入口模式',
      tokenEntryDesc: '使用 3 指令精简跳板替代默认入口，减小代码体积占用。'
    },
    logs: {
      title: '系统更新记录 / Release Notes'
    }
  }
}

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  fallbackLocale: 'en',
  messages,
})

export default i18n
