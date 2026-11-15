package assessment

func DefaultAllowedFileTypes() []AllowedFileType {
	return []AllowedFileType{
		{Extension: "csv", MediaType: "text/plain"},
		{Extension: "docx", MediaType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{Extension: "jpeg", MediaType: "image/jpeg"},
		{Extension: "jpg", MediaType: "image/jpeg"},
		{Extension: "pdf", MediaType: "application/pdf"},
		{Extension: "png", MediaType: "image/png"},
		{Extension: "pptx", MediaType: "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
		{Extension: "txt", MediaType: "text/plain"},
		{Extension: "xlsx", MediaType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
	}
}
