#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""将 PNG 图标转换为 ICO 格式"""

from PIL import Image
import os

# 源文件和目标文件路径
source_png = 'frontend/public/logo.png'
target_ico = 'build/logo.ico'

# 确保目标目录存在
os.makedirs('build', exist_ok=True)

# 打开源图片
img = Image.open(source_png)

# 定义 ICO 文件包含的尺寸
sizes = [(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]

# 转换并保存为 ICO
img.save(target_ico, format='ICO', sizes=sizes)

print(f'Successfully created: {target_ico}')
print(f'Source: {source_png}')
print(f'Sizes: {sizes}')
