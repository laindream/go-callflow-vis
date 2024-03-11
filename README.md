## go-callflow-vis

go-callflow-vis是一个命令行工具，专为分析Golang项目中指定函数的调用关系而设计。它通过分析代码来识别不同层级之间的函数调用关系，并生成一个多层二分图，从而帮助开发者理解和优化他们的代码结构。

#### 核心概念

go-callflow-vis的一个核心概念是调用层级。例如，在一个常见的HTTP业务服务端中，一个可能的调用层级是：

ApiHandle (API抽象)->

BizOperator (业务流程抽象)->

BizManager (业务对象抽象)->

DAO (数据访问层抽象)

此工具允许你指定每一层的核心函数，包括每个调用层级中需要包含的函数或一类函数。

#### 特性

多层二分图输出：显示相邻层函数之间的可达性，包括两个可达函数之间的可能调用路径。
灵活性：允许用户自定义每一层的关键函数或函数类别，以便更精确地分析项目结构。
优化辅助：通过可视化调用路径，帮助开发者识别和优化代码结构。

#### 安装

##### 使用go install进行安装

go install github.com/laindream/go-callflow-vis@latest
