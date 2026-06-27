package index

import "time"

// Entry representa um arquivo ou diretório indexado.
type Entry struct {
	Path    string // caminho relativo à raiz indexada
	Size    int64
	ModTime time.Time
	Mode    uint32 // permissões
	Hash    string // SHA-256 em hexadecimal (vazio se não calculado)
	IsDir   bool
}

// Index contém o conjunto de entradas e índices auxiliares.
type Index struct {
	Version   int    // versão do formato de índice
	RootPath  string // diretório raiz indexado
	CreatedAt time.Time
	Entries   []Entry          // lista ordenada por Path
	HashMap   map[string][]int // hash -> índices em Entries (para deduplicação)
	PathMap   map[string]int   // caminho relativo -> índice em Entries (acesso O(1))
}

// Query descreve critérios de busca.
type Query struct {
	Name       string // padrão glob para nome/caminho
	MinSize    int64
	MaxSize    int64 // 0 = sem limite superior
	Hash       string
	Duplicates bool // se true, retorna apenas duplicatas
	Limit      int  // número máximo de resultados (0 = sem limite)
	Offset     int  // início da página
}

// FindDuplicates retorna grupos de entradas que compartilham o mesmo hash.
func (idx *Index) FindDuplicates() [][]Entry {
	var groups [][]Entry
	for _, indices := range idx.HashMap {
		if len(indices) > 1 {
			group := make([]Entry, len(indices))
			for i, entryIdx := range indices {
				group[i] = idx.Entries[entryIdx]
			}
			groups = append(groups, group)
		}
	}
	return groups
}
