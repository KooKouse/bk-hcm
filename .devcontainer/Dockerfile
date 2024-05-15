# 使用包含Python 3.6的基础镜像
FROM python:3.6

# 设置工作目录
WORKDIR /app

# 安装Go（这可能需要从Debian或Alpine等的基础镜像开始）
# 但由于Python和Go的依赖和库可能不同，建议分别构建两个镜像
# 如果你只是想在Python环境中使用Go命令行工具，可以这样做：
# 注意：这里只是一个示例，实际安装可能需要更多步骤
RUN wget -qO- https://dl.google.com/go/go1.22.0.linux-amd64.tar.gz | tar -xzf - -C /usr/local
ENV PATH=$PATH:/usr/local/go/bin

# 将应用的源代码复制到工作目录中（如果有的话）
COPY . .