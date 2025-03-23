# PDF Searcher

## Overview

PDF Searcher is a **cross-platform desktop application** designed to search for keywords and phrases within multiple PDF documents efficiently. The application is built using **Vite + Electron** for the frontend, **Vue.js 3** for a modern UI, and **Python** for handling backend operations such as text extraction and searching.

This project uses a **portable Python environment (WinPython)** to ensure a seamless installation and execution without requiring a separate Python setup on the user's machine.

---

## Features

- **Multi-PDF Search**: Search for words and phrases across multiple PDFs.
- **Standalone Application**: Runs independently with a portable Python installation.
- **Modern UI**: Built with Vue.js 3 using `script setup` for better performance.
- **Electron-Based**: Packaged as a desktop application with Electron and Vite.
- **Python Backend**: Handles PDF text extraction and searching efficiently.
- **Automatic Setup**: Includes a PowerShell script to automate installation.

---

## Tech Stack

### Frontend:
- **[Vue.js 3](https://vuejs.org/)** – Reactive frontend framework.
- **[Vite](https://vitejs.dev/)** – Fast development environment.
- **[Electron](https://www.electronjs.org/)** – Desktop application framework.

### Backend:
- **[Python 3.11 (WinPython)](https://winpython.github.io/)**
- **[PyMuPDF (fitz)](https://pymupdf.readthedocs.io/en/latest/)** – PDF text extraction.

### Communication:
- **Electron IPC (Inter-Process Communication)** – For Vue.js and Python backend interaction.

---

## Installation & Setup

### Prerequisites:
Ensure you are using **Windows** and have:
- **PowerShell 5.1+** (Default in Windows)
- **WinPython Portable Installer (`Winpython64-3.11.3.1dot.exe`)**  
  _(Place it in the same folder as the setup script)_

### Clone the Repository:
```sh
git clone https://github.com/yourusername/pdf-searcher.git
cd pdf-searcher
