"""
License SDK - Python 3
Setup script for installing the SDK as a Python package
"""

from setuptools import setup, find_packages

setup(
    name='license-sdk',
    version='1.0.0',
    description='License SDK for Python 3 - Offline authorization file verification',
    long_description=open('README.md', 'r', encoding='utf-8').read(),
    long_description_content_type='text/markdown',
    author='License System Development Team',
    packages=find_packages(),
    install_requires=[
        'cryptography>=3.4.8',
        'requests>=2.28.0',
    ],
    python_requires='>=3.7',
    classifiers=[
        'Development Status :: 4 - Beta',
        'Intended Audience :: Developers',
        'License :: OSI Approved :: MIT License',
        'Programming Language :: Python :: 3',
        'Programming Language :: Python :: 3.7',
        'Programming Language :: Python :: 3.8',
        'Programming Language :: Python :: 3.9',
        'Programming Language :: Python :: 3.10',
        'Programming Language :: Python :: 3.11',
        'Topic :: Software Development :: Libraries :: Python Modules',
        'Topic :: Security :: Cryptography',
    ],
    keywords='license, authorization, offline, rsa, signature, verification',
)
