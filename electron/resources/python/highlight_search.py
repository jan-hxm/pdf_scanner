import pymupdf  # PyMuPDF
import os
import sys
import json
import subprocess

def open_pdf_at_page(pdf_path, page_number):
    if os.name == 'posix':  # Linux or MacOS
        subprocess.run(['xdg-open', pdf_path])  # Standard-PDF-Viewer
    elif os.name == 'nt':  # Windows
        standard_paths = [
            r"C:\Program Files (x86)\Adobe\Acrobat Reader DC\Reader\AcroRd32.exe",  # 32-Bit Standardpfad
            r"C:\Program Files\Adobe\Acrobat Reader DC\Reader\Acrobat.exe",       # 64-Bit Standardpfad
            r"C:\Program Files (x86)\Adobe\Acrobat DC\Acrobat\AcroRd32.exe",       # 32-Bit Acrobat DC
            r"C:\Program Files\Adobe\Acrobat DC\Acrobat\Acrobat.exe"              # 64-Bit Acrobat DC
        ]

        def find_acrobat():
            for path in standard_paths:
                if os.path.exists(path):
                    return path
            return None

        acrobat_path = find_acrobat()
        subprocess.run([acrobat_path, '/A', f'page={page_number}', pdf_path], shell=True)

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(json.dumps({"error": "Usage: script.py <pdf_path> <keyword>"}), file=sys.stderr, flush=True)
        sys.exit(1)

    pdf_path = sys.argv[1]
    page_num = sys.argv[2]

    print(json.dumps({"page": page_num}), flush=True)
    open_pdf_at_page(pdf_path, page_num)