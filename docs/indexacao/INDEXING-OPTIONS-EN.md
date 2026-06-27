# INDEXING-OPTIONS.md

## Introduction

The FastCopy project is a high-speed file copying tool that aims to provide an efficient and reliable solution for copying files. In this document, the indexing options that can be used to optimize searches and improve the tool's efficiency are presented.

## Indexing Options

The following are the indexing options that can be used in the FastCopy project:

1. **File Index**: Creates a file index that allows searching for files by name, size, creation date, etc.
2. **Content Index**: Creates a content index that allows searching for files by keywords, phrases, etc.
3. **Metadata Index**: Creates a metadata index that allows searching for files by metadata such as author, title, description, etc.
4. **Hash Index**: Creates a hash index that allows searching for files by content hash.

## Indexing Techniques

The following are the indexing techniques that can be used in the FastCopy project:

1. **Term Indexing**: Creates a term index that allows searching for files by keywords.
2. **Phrase Indexing**: Creates a phrase index that allows searching for files by phrases.
3. **Prefix Indexing**: Creates a prefix index that allows searching for files by keyword prefixes.
4. **Suffix Indexing**: Creates a suffix index that allows searching for files by keyword suffixes.

## Data Structures

The following are the data structures that can be used to store the indices:

1. **B-Tree**: A B-Tree is a data structure that allows storing and searching information efficiently.
2. **Prefix Tree (Trie)**: A prefix tree is a data structure that allows storing and searching keyword prefixes efficiently.
3. **Hash Table**: A hash table is a data structure that allows storing and searching information efficiently.

## Usage Examples

The following are examples of how to use the indexing options in the FastCopy project:

```bash
# Create a file index
fastcopy -i file -o file_index.db

# Search for files by name
fastcopy -s name -i file_index.db

# Create a content index
fastcopy -i content -o content_index.db

# Search for files by keyword
fastcopy -s keyword -i content_index.db
```
