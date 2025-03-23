import pymupdf
import os
import sys
import re
import json
from concurrent.futures import ProcessPoolExecutor
from rapidfuzz import fuzz

# Improved text normalization specifically for scientific terms
def normalize_text(text):
    """Standard text normalization removing whitespace and punctuation"""
    return re.sub(r"\s+|\.|,", "", text.lower().strip())

def normalize_scientific_name(text):
    """Specialized normalization for scientific names like E. Coli"""
    # Remove spaces, periods between single letters and words
    normalized = text.lower().strip()
    # Handle cases like "E. Coli" → match with "ecoli"
    normalized = re.sub(r'([a-z])\.\s*([a-z])', r'\1\2', normalized)
    # Remove remaining spaces and periods
    normalized = re.sub(r'[\s.]+', '', normalized)
    return normalized

def tokenize_text(line):
    """Enhanced tokenization that handles scientific notation and abbreviations"""
    # Basic words
    words = re.findall(r"\b[A-Za-z]+(?:['.-]\w+)*\b", line)
    
    # Scientific notation (e.g., "E. Coli")
    scientific = re.findall(r"\b[A-Za-z]\.\s?[A-Za-z][a-z]+\b", line)
    
    # Multi-word phrases
    phrases = re.findall(r"\b(?:\w+[\s.-])+\w+\b", line)
    
    # Abbreviations with periods
    abbreviations = re.findall(r"\b(?:[A-Za-z]\.){1,}[A-Za-z]?[a-z]*\b", line)
    
    return words + scientific + phrases + abbreviations

def generate_ngrams(text, n=3):
    """Generate character n-grams from text for fuzzy matching"""
    if len(text) < n:
        return [text]
    return [text[i:i+n] for i in range(len(text)-n+1)]

def ngram_similarity(text1, text2, n=3):
    """Calculate n-gram similarity between two strings"""
    # Normalize both texts
    norm_text1 = normalize_scientific_name(text1)
    norm_text2 = normalize_scientific_name(text2)
    
    # Generate n-grams
    ngrams1 = set(generate_ngrams(norm_text1, n))
    ngrams2 = set(generate_ngrams(norm_text2, n))
    
    if not ngrams2:
        return 0
    
    # Calculate Jaccard similarity
    intersection = len(ngrams1.intersection(ngrams2))
    union = len(ngrams1.union(ngrams2))
    
    if union == 0:
        return 0
        
    return (intersection / union) * 100

def advanced_match(line, keyword):
    """Multi-strategy matching approach for robust fuzzy search"""
    # Try exact matching (case insensitive)
    line_lower = line.lower()
    keyword_lower = keyword.lower()
    
    # Direct exact match
    if keyword_lower in line_lower:
        match = re.search(re.escape(keyword), line, re.IGNORECASE)
        if match:
            return 100, match.group()
    
    # Scientific name normalized matching
    norm_line = normalize_scientific_name(line)
    norm_keyword = normalize_scientific_name(keyword)
    
    if norm_keyword in norm_line:
        # Try to find the original form in the text
        # This handles cases like "E. Coli" matching "ecoli"
        parts = keyword.replace('.', '').replace(' ', '')
        pattern = ''
        for c in parts:
            pattern += f'[{c.upper()}{c.lower()}]\.?\s*'
        pattern = pattern.rstrip('\\s*')
        
        matches = re.finditer(f"{pattern}", line)
        for match in matches:
            if normalize_scientific_name(match.group()) == norm_keyword:
                return 95, match.group()
    
    # Tokenized matching
    words = tokenize_text(line)
    for word in words:
        if normalize_scientific_name(word) == norm_keyword:
            return 90, word
    
    # Fuzzy matching with multiple strategies
    best_score = 0
    best_match = None
    
    # Token-based fuzzy matching
    for word in words:
        if len(word) > 2:  # Avoid matching very short words
            # Try different fuzzy algorithms and take the best score
            ratio = fuzz.ratio(normalize_scientific_name(word), norm_keyword)
            partial = fuzz.partial_ratio(normalize_scientific_name(word), norm_keyword)
            score = max(ratio, partial)
            
            if score > best_score:
                best_score = score
                best_match = word
    
    # N-gram similarity as a backup
    ngram_score = ngram_similarity(line, keyword)
    if ngram_score > best_score:
        # For n-gram matches, extract the most relevant part of the text
        tokens = line.split()
        ngram_match = None
        best_ngram_score = 0
        
        # Look for the best matching span of up to 3 words
        for i in range(len(tokens)):
            for span in range(1, min(4, len(tokens) - i + 1)):
                phrase = " ".join(tokens[i:i+span])
                score = ngram_similarity(phrase, keyword)
                if score > best_ngram_score:
                    best_ngram_score = score
                    ngram_match = phrase
        
        best_score = ngram_score
        best_match = ngram_match if ngram_match else best_match
    
    return best_score, best_match

def process_pdf_with_fitz(pdf_path, keyword):
    """Process a PDF file searching for keyword matches"""
    results = []
    seen = set()

    try:
        doc = pymupdf.open(pdf_path)
        file_name = os.path.basename(pdf_path)

        for page_num, page in enumerate(doc.pages()):
            text = page.get_text("text")
            lines = text.split("\n")

            for line in lines:
                if len(line) > 1:
                    # Use the advanced matching algorithm
                    similarity, found_word = advanced_match(line, keyword)
                    
                    if similarity >= 85:  # Threshold for considering a match
                        key = (page_num + 1, found_word)

                        if key not in seen:
                            positions = []
                            # Handle search positioning differently for different match types
                            if found_word:
                                search_terms = [found_word]
                                
                                # If we have a scientific notation match
                                # Also look for variations of the found word
                                if re.search(r'[A-Za-z]\.\s?[A-Za-z]', found_word):
                                    # Add variation without spacing
                                    no_space = re.sub(r'([A-Za-z])\.\s+([A-Za-z])', r'\1.\2', found_word)
                                    search_terms.append(no_space)
                                    
                                    # Add variation without period
                                    no_period = re.sub(r'([A-Za-z])\.\s*([A-Za-z])', r'\1 \2', found_word)
                                    search_terms.append(no_period)
                                
                                # Look for all variations on the page
                                for term in search_terms:
                                    try:
                                        for rect in page.search_for(term):
                                            x0, y0, x1, y1 = rect
                                            positions.append({"x0": x0, "y0": y0, "x1": x1, "y1": y1})
                                    except:
                                        pass  # Skip failed searches
                            
                            if not positions and found_word:
                                # Fallback: search for individual words if phrase search fails
                                words = found_word.split()
                                for word in words:
                                    if len(word) > 2:  # Skip very short words
                                        try:
                                            for rect in page.search_for(word):
                                                x0, y0, x1, y1 = rect
                                                positions.append({"x0": x0, "y0": y0, "x1": x1, "y1": y1})
                                        except:
                                            pass

                            seen.add(key)
                            results.append({
                                "file": pdf_path,
                                "fileName": file_name,
                                "page": page_num + 1,
                                "found": True,
                                "match": "content",
                                "confidence": similarity,
                                "foundWord": found_word,
                                "positions": positions,
                                "context": line.strip()  # Adding context helps users verify matches
                            })
    except Exception as e:
        print(json.dumps({"error": f"Error processing {pdf_path}: {e}"}), file=sys.stderr, flush=True)

    return results

def search_pdfs_in_folder(folder_path, keyword):
    """Search for keyword in all PDFs in a folder"""
    results = []
    pdf_files = []
    
    # First collect all PDF files
    for root, dirs, files in os.walk(folder_path):
        for file in files:
            if file.lower().endswith(".pdf"):
                pdf_files.append(os.path.join(root, file))
    
    total_files = len(pdf_files)
    current_file = 0

    try:
        with ProcessPoolExecutor() as executor:
            future_to_pdf = {executor.submit(process_pdf_with_fitz, pdf_path, keyword): pdf_path 
                             for pdf_path in pdf_files}
            
            for future in future_to_pdf:
                result = future.result()
                if result:
                    results.extend(result)
                
                current_file += 1
                progress_update = {"progress": int((current_file / total_files) * 100)}
                print(json.dumps(progress_update), flush=True)
    
    except Exception as e:
        print(json.dumps({"error": f"Error searching PDFs: {e}"}), file=sys.stderr, flush=True)
        sys.exit(2)

    # Sort results by confidence score (highest first)
    results.sort(key=lambda x: x["confidence"], reverse=True)
    
    # Add an overall summary
    summary = {
        "total_files_searched": total_files,
        "files_with_matches": len(set(r["file"] for r in results)),
        "total_matches": len(results),
        "keyword": keyword
    }
    
    print(json.dumps({"results": results, "summary": summary}), flush=True)

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print(json.dumps({"error": "Usage: pdf_search.py <folder_path> <keyword>"}), file=sys.stderr, flush=True)
        sys.exit(2)

    folder_path = sys.argv[1]
    keyword = sys.argv[2]

    search_pdfs_in_folder(folder_path, keyword)