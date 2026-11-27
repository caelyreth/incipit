package code

import "strings"

// language is how a language writes what highlighting tells apart: its
// words of each class, its comments and its strings. It is data, so a
// language is added here and nowhere else.
type language struct {
	names []string
	// line starts a comment to the line's end; blocks open and close
	// comments that may span lines.
	line   []string
	blocks [][2]string
	// quotes open and close strings. A string ends at its line's end
	// unless its quote is in multi; one in raw takes no escapes. Triple
	// writes a string between three quotes, which spans lines.
	quotes, multi, raw string
	triple             bool
	// word are the characters besides letters, digits and '_' a word
	// holds; fold reads words whatever their case.
	word string
	fold bool

	keywords, types, constants string
}

var cFamily = language{
	line: []string{"//"}, blocks: [][2]string{{"/*", "*/"}}, quotes: `"'`,
}

var languages = []language{
	{names: []string{"go", "golang"}, line: cFamily.line, blocks: cFamily.blocks, quotes: "\"'`", multi: "`", raw: "`",
		keywords:  "break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var",
		types:     "any bool byte comparable complex64 complex128 error float32 float64 int int8 int16 int32 int64 rune string uint uint8 uint16 uint32 uint64 uintptr",
		constants: "true false nil iota"},
	{names: []string{"js", "javascript", "jsx", "mjs", "cjs", "ts", "typescript", "tsx"}, line: cFamily.line, blocks: cFamily.blocks,
		quotes: "\"'`", multi: "`", word: "$",
		keywords:  "abstract as async await break case catch class const continue debugger declare default delete do else enum export extends finally for from function get if implements import in instanceof interface let new of private protected public readonly return set static super switch this throw try type typeof var void while with yield",
		types:     "any bigint boolean never number object string symbol unknown void Array Map Promise Record Set",
		constants: "true false null undefined NaN Infinity"},
	{names: []string{"py", "python", "python3"}, line: []string{"#"}, quotes: `"'`, triple: true,
		keywords:  "and as assert async await break class continue def del elif else except finally for from global if import in is lambda match case nonlocal not or pass raise return try while with yield",
		types:     "bool bytes dict float int list object set str tuple",
		constants: "True False None self"},
	{names: []string{"rs", "rust"}, line: cFamily.line, blocks: cFamily.blocks, quotes: `"`, multi: `"`,
		keywords:  "as async await break const continue crate dyn else enum extern fn for if impl in let loop match mod move mut pub ref return static struct super trait type unsafe use where while",
		types:     "bool char f32 f64 i8 i16 i32 i64 i128 isize str u8 u16 u32 u64 u128 usize Self String Vec Option Result Box",
		constants: "true false None Some Ok Err self"},
	{names: []string{"c", "h", "cpp", "c++", "cc", "cxx", "hpp", "objc", "objective-c"}, line: cFamily.line, blocks: cFamily.blocks, quotes: cFamily.quotes, word: "#",
		keywords:  "auto break case catch class const constexpr continue default delete do else enum explicit extern for friend goto if inline namespace new noexcept operator private protected public register return sizeof static static_cast struct switch template this throw try typedef typename union using virtual volatile while #include #define #if #ifdef #ifndef #endif #else #pragma",
		types:     "bool char double float int long short signed unsigned void size_t int8_t int16_t int32_t int64_t uint8_t uint16_t uint32_t uint64_t std string vector",
		constants: "true false NULL nullptr"},
	{names: []string{"java", "kt", "kotlin", "scala"}, line: cFamily.line, blocks: cFamily.blocks, quotes: cFamily.quotes,
		keywords:  "abstract as break case catch class companion const continue data default do else enum extends final finally for fun if implements import in instanceof interface internal is lateinit new object open override package private protected public return sealed static super switch synchronized this throw throws try val var when while",
		types:     "boolean byte char double float int long short void Boolean Int Long String Unit Any List Map",
		constants: "true false null"},
	{names: []string{"swift"}, line: cFamily.line, blocks: cFamily.blocks, quotes: `"`, triple: true,
		keywords:  "actor as associatedtype async await break case catch class continue default defer do else enum extension fallthrough for func guard if import in init inout internal is let mutating nonisolated open operator private protocol public repeat rethrows return self some static struct subscript super switch throw throws try typealias var where while",
		types:     "Any Bool Character Double Float Int String Void Array Dictionary Optional Set",
		constants: "true false nil"},
	{names: []string{"sh", "bash", "zsh", "shell", "fish", "console"}, line: []string{"#"}, quotes: `"'`, multi: `"'`, raw: "'", word: "$-",
		keywords:  "case do done elif else esac export fi for function if in local return select set then unset until while echo cd exit source alias",
		constants: "true false"},
	{names: []string{"sql", "postgres", "postgresql", "sqlite", "mysql"}, line: []string{"--"}, blocks: cFamily.blocks, quotes: `'"`, fold: true,
		keywords:  "add all alter and as asc begin between by case check commit constraint create cross default delete desc distinct drop else end exists foreign from full group having if in index inner insert into is join key left like limit not on or order outer primary references returning right rollback select set table then transaction union unique update using values view when where with",
		types:     "bigint blob boolean char date decimal double float int integer json jsonb numeric real serial smallint text time timestamp uuid varchar",
		constants: "true false null"},
	{names: []string{"json", "jsonc", "json5"}, line: cFamily.line, blocks: cFamily.blocks, quotes: `"`,
		constants: "true false null"},
	{names: []string{"yaml", "yml"}, line: []string{"#"}, quotes: `"'`,
		constants: "true false null yes no on off"},
	{names: []string{"toml", "ini", "conf"}, line: []string{"#", ";"}, quotes: `"'`, triple: true,
		constants: "true false"},
	{names: []string{"css", "scss", "less"}, line: []string{"//"}, blocks: cFamily.blocks, quotes: `"'`, word: "-@!",
		keywords:  "@media @import @font-face @keyframes @supports @layer !important",
		constants: "inherit initial unset none auto"},
	{names: []string{"html", "xml", "svg", "svelte", "vue"}, blocks: [][2]string{{"<!--", "-->"}}, quotes: `"'`, word: "-"},
	{names: []string{"rb", "ruby"}, line: []string{"#"}, quotes: `"'`, word: "?!",
		keywords:  "alias and begin break case class def defined? do else elsif end ensure for if in module next not or redo rescue retry return self super then undef unless until when while yield require attr_accessor",
		constants: "true false nil"},
	{names: []string{"lua"}, line: []string{"--"}, quotes: `"'`,
		keywords:  "and break do else elseif end for function goto if in local not or repeat return then until while",
		constants: "true false nil"},
	{names: []string{"zig"}, line: []string{"//"}, quotes: `"'`,
		keywords:  "align and anytype asm break catch comptime const continue defer else enum errdefer error export extern fn for if inline orelse or packed pub return struct switch test try union unreachable usingnamespace var volatile while",
		types:     "bool f16 f32 f64 i8 i16 i32 i64 isize u8 u16 u32 u64 usize void type anyerror noreturn",
		constants: "true false null undefined"},
}

// lexicon is a language made ready to read: its words by class.
type lexicon struct {
	*language
	words map[string]class
}

var lexicons = func() map[string]*lexicon {
	out := map[string]*lexicon{}
	for i := range languages {
		l := &languages[i]
		lx := &lexicon{language: l, words: map[string]class{}}
		for c, list := range map[class]string{classKeyword: l.keywords, classType: l.types, classConstant: l.constants} {
			for _, w := range strings.Fields(list) {
				if l.fold {
					w = strings.ToLower(w)
				}
				lx.words[w] = c
			}
		}
		for _, n := range l.names {
			out[n] = lx
		}
	}
	return out
}()

// lexiconOf is the language an info string names, or nil.
func lexiconOf(lang string) *lexicon { return lexicons[lang] }
