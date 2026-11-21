package fonts

// Code generated from the pinned files' sizes and SHA-256 sums; DO NOT EDIT
// by hand: a face changes with its commit.

// Source is Google's font repository's ofl folder at the commit the
// catalog is pinned to.
const Source = "https://raw.githubusercontent.com/google/fonts/06bfe0d62a98375cb09c942fa18e68bd32cff057/ofl"

// Catalog is the faces the app offers, in the order the settings list them
// and the prose family list falls back through: the faces that draw Han
// characters first, then the other scripts.
var Catalog = []Face{
	{ID: "sc", Name: "Chinese, Simplified", Sample: "永和九年，岁在癸丑，暮春之初", Family: "Noto Serif SC", Han: true, Files: []File{
		{Path: "notoserifsc/NotoSerifSC[wght].ttf", Size: 25125512, SHA256: "050080d9255a86808f2945bffac582b31ef32bc36411ce29563b4961670c66f9"},
	}},
	{ID: "tc", Name: "Chinese, Traditional", Sample: "永和九年，歲在癸丑，暮春之初", Family: "Noto Serif TC", Han: true, Files: []File{
		{Path: "notoseriftc/NotoSerifTC[wght].ttf", Size: 16851596, SHA256: "0077e18f57c6908f4a000969880940bdb0dad057c0e8d98b49dc364c3d1b09c6"},
	}},
	{ID: "hk", Name: "Chinese, Hong Kong", Sample: "維港兩岸，燈火通明", Family: "Noto Serif HK", Han: true, Files: []File{
		{Path: "notoserifhk/NotoSerifHK[wght].ttf", Size: 16828640, SHA256: "66108860e321aa413e7cb346448b4a418b779d94ae392accb32d163c024ae661"},
	}},
	{ID: "jp", Name: "Japanese", Sample: "いろはにほへと\u3000ちりぬるを", Family: "Noto Serif JP", Han: true, Files: []File{
		{Path: "notoserifjp/NotoSerifJP[wght].ttf", Size: 13574352, SHA256: "2fd527ba12b6a44ec30d796d633360da0aeba6c5d4af1304ce12bb4dc15a7dfc"},
	}},
	{ID: "kr", Name: "Korean", Sample: "다람쥐 헌 쳇바퀴에 타고파", Family: "Noto Serif KR", Han: true, Files: []File{
		{Path: "notoserifkr/NotoSerifKR[wght].ttf", Size: 23795420, SHA256: "11f8d5de6f1b79195efba3828aaa2ec95c1178f5ae976fb23c8d53250a9938f3"},
	}},
	{ID: "thai", Name: "Thai", Sample: "เป็นมนุษย์สุดประเสริฐเลิศคุณค่า", Family: "Noto Serif Thai", Han: false, Files: []File{
		{Path: "notoserifthai/NotoSerifThai[wdth,wght].ttf", Size: 292188, SHA256: "34a7ad11647c845303aabdde639059806c56b84719e5d2ceb28eb038711bdf53"},
	}},
	{ID: "lao", Name: "Lao", Sample: "ສະບາຍດີ", Family: "Noto Serif Lao", Han: false, Files: []File{
		{Path: "notoseriflao/NotoSerifLao[wdth,wght].ttf", Size: 217332, SHA256: "c35bb3fbbdbd88c0d8ce5ee13e4854cb51471d9b809108570876b48c8f970df6"},
	}},
	{ID: "khmer", Name: "Khmer", Sample: "សួស្តី\u200bពិភពលោក", Family: "Noto Serif Khmer", Han: false, Files: []File{
		{Path: "notoserifkhmer/NotoSerifKhmer[wdth,wght].ttf", Size: 498288, SHA256: "50ad032042491e98b440e41490d26b716d8d43886eebfafa9735f6284c1980e2"},
	}},
	{ID: "myanmar", Name: "Myanmar", Sample: "မင်္ဂလာပါ", Family: "Noto Serif Myanmar", Han: false, Files: []File{
		{Path: "notoserifmyanmar/NotoSerifMyanmar-Regular.ttf", Size: 260396, SHA256: "f88d1da5a75a8e09ff0b073fe3c861d095a72e94fea3235f09f80e6c5b2c9933"},
		{Path: "notoserifmyanmar/NotoSerifMyanmar-Bold.ttf", Size: 270276, SHA256: "4188186da169f55df6cba7b01b9e08c9f44516e0ad66766fc8b46e806b423a7e"},
	}},
	{ID: "tibetan", Name: "Tibetan", Sample: "བཀྲ་ཤིས་བདེ་ལེགས།", Family: "Noto Serif Tibetan", Han: false, Files: []File{
		{Path: "notoseriftibetan/NotoSerifTibetan[wght].ttf", Size: 2063504, SHA256: "060ec022b04c306de3f58d051fb0e1cf81a5b610c5910fbfc43bba154c057cda"},
	}},
	{ID: "arabic", Name: "Arabic", Sample: "مرحبا بالعالم", Family: "Noto Naskh Arabic", Han: false, Files: []File{
		{Path: "notonaskharabic/NotoNaskhArabic[wght].ttf", Size: 307592, SHA256: "67b5a525a661b607971fbd3f96a81b89d3a768e74534fca84f18ac97e6fab72f"},
	}},
	{ID: "hebrew", Name: "Hebrew", Sample: "שלום עולם", Family: "Noto Serif Hebrew", Han: false, Files: []File{
		{Path: "notoserifhebrew/NotoSerifHebrew[wdth,wght].ttf", Size: 183600, SHA256: "93caef921360788dc3b0e32136bb26f16bc57717ec482d48fc7fd43820617165"},
	}},
	{ID: "armenian", Name: "Armenian", Sample: "Բարեւ աշխարհ", Family: "Noto Serif Armenian", Han: false, Files: []File{
		{Path: "notoserifarmenian/NotoSerifArmenian[wdth,wght].ttf", Size: 292528, SHA256: "9354b74384d093d59f9a4ad958e188808d58b6f15debeaab18e07bc432f8ce0c"},
	}},
	{ID: "georgian", Name: "Georgian", Sample: "გამარჯობა მსოფლიო", Family: "Noto Serif Georgian", Han: false, Files: []File{
		{Path: "notoserifgeorgian/NotoSerifGeorgian[wdth,wght].ttf", Size: 381592, SHA256: "a13a90c44a612ab80f79032d3edbbc7f6757f0c6b1613dba1a92f38077cccc57"},
	}},
	{ID: "ethiopic", Name: "Ethiopic", Sample: "ሰላም ለዓለም", Family: "Noto Serif Ethiopic", Han: false, Files: []File{
		{Path: "notoserifethiopic/NotoSerifEthiopic[wdth,wght].ttf", Size: 1057996, SHA256: "49d5daa6c3477a427f5d4f8a5ae9ea7f0a04ea7f865fc21f7e08fc4aed70d92a"},
	}},
	{ID: "devanagari", Name: "Devanagari", Sample: "नमस्ते दुनिया", Family: "Noto Serif Devanagari", Han: false, Files: []File{
		{Path: "notoserifdevanagari/NotoSerifDevanagari[wdth,wght].ttf", Size: 757692, SHA256: "1191e07bfeb062d80e252eb85b0eafdfbda1e350707a2a60628668e8f677dbbb"},
	}},
	{ID: "bengali", Name: "Bengali", Sample: "ওহে বিশ্ব", Family: "Noto Serif Bengali", Han: false, Files: []File{
		{Path: "notoserifbengali/NotoSerifBengali[wdth,wght].ttf", Size: 981812, SHA256: "083180a7b47cf3b101ed55a8724aa1b38869daa9baf23b7cf198f9d225365aba"},
	}},
	{ID: "gurmukhi", Name: "Gurmukhi", Sample: "ਸਤਿ ਸ੍ਰੀ ਅਕਾਲ", Family: "Noto Serif Gurmukhi", Han: false, Files: []File{
		{Path: "notoserifgurmukhi/NotoSerifGurmukhi[wght].ttf", Size: 144664, SHA256: "0d15ba8362035a032156c0516038cb8ed93507f7a7762df3e65eaad9218d764b"},
	}},
	{ID: "gujarati", Name: "Gujarati", Sample: "નમસ્તે દુનિયા", Family: "Noto Serif Gujarati", Han: false, Files: []File{
		{Path: "notoserifgujarati/NotoSerifGujarati[wght].ttf", Size: 378356, SHA256: "b76ad2086be5faa06a2e0d89e2ff58691d3754d1bb20f9dff0d7b10516c18831"},
	}},
	{ID: "oriya", Name: "Odia", Sample: "ନମସ୍କାର ଦୁନିଆ", Family: "Noto Serif Oriya", Han: false, Files: []File{
		{Path: "notoseriforiya/NotoSerifOriya[wght].ttf", Size: 241144, SHA256: "9fa2a08cb6a677d155a823bc58b4c587613780bd43013fd362369c251001502a"},
	}},
	{ID: "tamil", Name: "Tamil", Sample: "வணக்கம் உலகம்", Family: "Noto Serif Tamil", Han: false, Files: []File{
		{Path: "notoseriftamil/NotoSerifTamil[wdth,wght].ttf", Size: 420124, SHA256: "4b10fe56ee22907a80136c16dbe386ee20f3bf116332b35128bb35eeb775d91e"},
	}},
	{ID: "telugu", Name: "Telugu", Sample: "నమస్కారం ప్రపంచం", Family: "Noto Serif Telugu", Han: false, Files: []File{
		{Path: "notoseriftelugu/NotoSerifTelugu[wght].ttf", Size: 543100, SHA256: "32210d8d514d9aad43bf927fbd47c7e3066ee6a20c0258678f2ee45915012a26"},
	}},
	{ID: "kannada", Name: "Kannada", Sample: "ನಮಸ್ಕಾರ ಜಗತ್ತು", Family: "Noto Serif Kannada", Han: false, Files: []File{
		{Path: "notoserifkannada/NotoSerifKannada[wght].ttf", Size: 495384, SHA256: "3d97c98dd59251a85fcc6e95edf9be02b60147e92486927390247f1cd4d5eb37"},
	}},
	{ID: "malayalam", Name: "Malayalam", Sample: "നമസ്കാരം ലോകം", Family: "Noto Serif Malayalam", Han: false, Files: []File{
		{Path: "notoserifmalayalam/NotoSerifMalayalam[wght].ttf", Size: 242048, SHA256: "eed354cadb35992df6e8b32fdb7edb399989b165f0f25e7a921009683db26e8e"},
	}},
	{ID: "sinhala", Name: "Sinhala", Sample: "ආයුබෝවන් ලෝකය", Family: "Noto Serif Sinhala", Han: false, Files: []File{
		{Path: "notoserifsinhala/NotoSerifSinhala[wdth,wght].ttf", Size: 1026928, SHA256: "2b0df6b8bde56d3b934ba89fada82486b1ec50ee06842c81be36496f8c9b5ac9"},
	}},
}
