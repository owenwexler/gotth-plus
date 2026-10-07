package styles

// Tailwind only generates classes it can find as complete literal strings, so every class here is
// spelled out in full (static/input.css lists this file as a source).

func GenerateBigButtonClasses() string {
	return "bg-black text-white px-4 py-2 rounded-full border border-solid border-black text-sm font-medium cursor-pointer hover:bg-black/80 active:scale-95 transition-transform duration-[10ms] disabled:cursor-wait"
}

func GenerateSmallButtonClasses() string {
	return "bg-black text-white px-4 py-1 rounded-full border border-solid border-black text-sm font-medium cursor-pointer hover:bg-black/80 active:scale-95 transition-transform duration-[10ms] disabled:cursor-wait"
}

func GenerateIconButtonClasses() string {
	return "text-black hover:text-gray-500 focus:outline-none cursor-pointer active:scale-95 transition-transform duration-[10ms]"
}

func GenerateSVGImageClasses() string {
	return "h-5 w-5 fill-black hover:fill-gray-500"
}

func GenerateLinkButtonClasses() string {
	return "bg-transparent text-black underline cursor-pointer active:scale-95 transition-transform duration-[10ms]"
}

func GenerateFormDivClasses() string {
	return "flex flex-col space-y-2 items-center justify-center pb-2"
}

func GenerateHorizontalFormClasses() string {
	return "flex items-center justify-between gap-4 bg-white text-black px-1 py-2.5 rounded-lg font-sans"
}

func GenerateInputClasses() string {
	return "flex-1 bg-transparent p-2 text-black placeholder-black/40 outline-black border border-solid border-black text-base"
}

func GenerateDialogClasses() string {
	return "overflow-visible fixed inset-0 z-50 m-auto p-6 rounded-lg shadow-xl backdrop:bg-gray-900/50"
}

func GenerateCardClasses() string {
	return "w-full max-w-md flex flex-col items-center gap-4 rounded-lg border border-solid border-black p-6"
}
