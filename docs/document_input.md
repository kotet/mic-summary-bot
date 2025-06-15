# Document understanding

-   On this page
-   [PDF input](https://ai.google.dev/gemini-api/docs/document-processing?lang=go#pdf-input)
    -   [As inline data](https://ai.google.dev/gemini-api/docs/document-processing?lang=go#inline_data)
    -   [Locally stored PDFs](https://ai.google.dev/gemini-api/docs/document-processing?lang=go#local-pdfs)
    -   [Large PDFs](https://ai.google.dev/gemini-api/docs/document-processing?lang=go#large-pdfs)
    -   [Multiple PDFs](https://ai.google.dev/gemini-api/docs/document-processing?lang=go#prompt-multiple)
-   [Technical details](https://ai.google.dev/gemini-api/docs/document-processing?lang=go#technical-details)
-   [What's next](https://ai.google.dev/gemini-api/docs/document-processing?lang=go#whats-next)

The Gemini API supports PDF input, including long documents (up to 1000 pages). Gemini models process PDFs with native vision, and are therefore able to understand both text and image contents inside documents. With native PDF vision support, Gemini models are able to:

-   Analyze diagrams, charts, and tables inside documents
-   Extract information into structured output formats
-   Answer questions about visual and text contents in documents
-   Summarize documents
-   Transcribe document content (e.g. to HTML) preserving layouts and formatting, for use in downstream applications

This tutorial demonstrates some possible ways to use the Gemini API to process PDF documents.

## PDF input

For PDF payloads under 20MB, you can choose between uploading base64 encoded documents or directly uploading locally stored files.

### As inline data

You can process PDF documents directly from URLs. Here's a code snippet showing how to do this:

package main
    
    import (
        "context"
        "fmt"
        "io"
        "net/http"
        "os"
        "google.golang.org/genai"
    )
    
    func main() {
    
        ctx := context.Background()
        client, _ := genai.NewClient(ctx, &genai.ClientConfig{
            APIKey:  os.Getenv("GEMINI_API_KEY"),
            Backend: genai.BackendGeminiAPI,
        })
    
        pdfResp, _ := http.Get("https://discovery.ucl.ac.uk/id/eprint/10089234/1/343019_3_art_0_py4t4l_convrt.pdf")
        var pdfBytes []byte
        if pdfResp != nil && pdfResp.Body != nil {
            pdfBytes, _ = io.ReadAll(pdfResp.Body)
            pdfResp.Body.Close()
        }
    
        parts := []*genai.Part{
            &genai.Part{
                InlineData: &genai.Blob{
                    MIMEType: "application/pdf",
                    Data:     pdfBytes,
                },
            },
            genai.NewPartFromText("Summarize this document"),
        }
    
        contents := []*genai.Content{
            genai.NewContentFromParts(parts, genai.RoleUser),
        }
    
        result, _ := client.Models.GenerateContent(
            ctx,
            "gemini-2.0-flash",
            contents,
            nil,
        )
    
        fmt.Println(result.Text())
    }

### Locally stored PDFs

For locally stored PDFs, you can use the following approach:

package main
    
    import (
        "context"
        "fmt"
        "os"
        "google.golang.org/genai"
    )
    
    func main() {
    
        ctx := context.Background()
        client, _ := genai.NewClient(ctx, &genai.ClientConfig{
            APIKey:  os.Getenv("GEMINI_API_KEY"),
            Backend: genai.BackendGeminiAPI,
        })
    
        pdfBytes, _ := os.ReadFile("path/to/your/file.pdf")
    
        parts := []*genai.Part{
            &genai.Part{
                InlineData: &genai.Blob{
                    MIMEType: "application/pdf",
                    Data:     pdfBytes,
                },
            },
            genai.NewPartFromText("Summarize this document"),
        }
        contents := []*genai.Content{
            genai.NewContentFromParts(parts, genai.RoleUser),
        }
    
        result, _ := client.Models.GenerateContent(
            ctx,
            "gemini-2.0-flash",
            contents,
            nil,
        )
    
        fmt.Println(result.Text())
    }

### Large PDFs

You can use the File API to upload larger documents. Always use the File API when the total request size (including the files, text prompt, system instructions, etc.) is larger than 20 MB.

**Note:** The File API lets you store up to 50 MB of PDF files. Files are stored for 48 hours. They can be accessed in that period with your API key, but cannot be downloaded from the API. The File API is available at no cost in all regions where the Gemini API is available.

Call [`media.upload`](https://ai.google.dev/api/rest/v1beta/media/upload) to upload a file using the File API. The following code uploads a document file and then uses the file in a call to [`models.generateContent`](https://ai.google.dev/api/generate-content#method:-models.generatecontent).

#### Large PDFs from URLs

Use the File API for large PDF files available from URLs, simplifying the process of uploading and processing these documents directly through their URLs:

package main
    
    import (
        "context"
        "fmt"
        "io"
        "net/http"
        "os"
        "google.golang.org/genai"
    )
    
    func main() {
    
        ctx := context.Background()
        client, _ := genai.NewClient(ctx, &genai.ClientConfig{
            APIKey:  os.Getenv("GEMINI_API_KEY"),
            Backend: genai.BackendGeminiAPI,
        })
    
        pdfURL := "https://www.nasa.gov/wp-content/uploads/static/history/alsj/a17/A17_FlightPlan.pdf"
        localPdfPath := "A17_FlightPlan_downloaded.pdf"
    
        respHttp, _ := http.Get(pdfURL)
        defer respHttp.Body.Close()
    
        outFile, _ := os.Create(localPdfPath)
        defer outFile.Close()
    
        _, _ = io.Copy(outFile, respHttp.Body)
    
        uploadConfig := &genai.UploadFileConfig{MIMEType: "application/pdf"}
        uploadedFile, _ := client.Files.UploadFromPath(ctx, localPdfPath, uploadConfig)
    
        promptParts := []*genai.Part{
            genai.NewPartFromURI(uploadedFile.URI, uploadedFile.MIMEType),
            genai.NewPartFromText("Summarize this document"),
        }
        contents := []*genai.Content{
            genai.NewContentFromParts(promptParts, genai.RoleUser), // Specify role
        }
    
        result, _ := client.Models.GenerateContent(
            ctx,
            "gemini-2.0-flash",
            contents,
            nil,
        )
    
        fmt.Println(result.Text())
    }

#### Large PDFs stored locally

    package main
    
        import (
            "context"
            "fmt"
            "os"
            "google.golang.org/genai"
        )
    
        func main() {
    
            ctx := context.Background()
            client, _ := genai.NewClient(ctx, &genai.ClientConfig{
                APIKey:  os.Getenv("GEMINI_API_KEY"),
                Backend: genai.BackendGeminiAPI,
            })
            localPdfPath := "/path/to/file.pdf"
    
            uploadConfig := &genai.UploadFileConfig{MIMEType: "application/pdf"}
            uploadedFile, _ := client.Files.UploadFromPath(ctx, localPdfPath, uploadConfig)
    
            promptParts := []*genai.Part{
                genai.NewPartFromURI(uploadedFile.URI, uploadedFile.MIMEType),
                genai.NewPartFromText("Give me a summary of this pdf file."),
            }
            contents := []*genai.Content{
                genai.NewContentFromParts(promptParts, genai.RoleUser),
            }
    
            result, _ := client.Models.GenerateContent(
                ctx,
                "gemini-2.0-flash",
                contents,
                nil,
            )
    
            fmt.Println(result.Text())
        }

### Multiple PDFs

The Gemini API is capable of processing multiple PDF documents in a single request, as long as the combined size of the documents and the text prompt stays within the model's context window.

    package main
    
        import (
            "context"
            "fmt"
            "io"
            "net/http"
            "os"
            "google.golang.org/genai"
        )
    
        func main() {
    
            ctx := context.Background()
            client, _ := genai.NewClient(ctx, &genai.ClientConfig{
                APIKey:  os.Getenv("GEMINI_API_KEY"),
                Backend: genai.BackendGeminiAPI,
            })
    
            docUrl1 := "https://arxiv.org/pdf/2312.11805"
            docUrl2 := "https://arxiv.org/pdf/2403.05530"
            localPath1 := "doc1_downloaded.pdf"
            localPath2 := "doc2_downloaded.pdf"
    
            respHttp1, _ := http.Get(docUrl1)
            defer respHttp1.Body.Close()
    
            outFile1, _ := os.Create(localPath1)
            _, _ = io.Copy(outFile1, respHttp1.Body)
            outFile1.Close()
    
            respHttp2, _ := http.Get(docUrl2)
            defer respHttp2.Body.Close()
    
            outFile2, _ := os.Create(localPath2)
            _, _ = io.Copy(outFile2, respHttp2.Body)
            outFile2.Close()
    
            uploadConfig1 := &genai.UploadFileConfig{MIMEType: "application/pdf"}
            uploadedFile1, _ := client.Files.UploadFromPath(ctx, localPath1, uploadConfig1)
    
            uploadConfig2 := &genai.UploadFileConfig{MIMEType: "application/pdf"}
            uploadedFile2, _ := client.Files.UploadFromPath(ctx, localPath2, uploadConfig2)
    
            promptParts := []*genai.Part{
                genai.NewPartFromURI(uploadedFile1.URI, uploadedFile1.MIMEType),
                genai.NewPartFromURI(uploadedFile2.URI, uploadedFile2.MIMEType),
                genai.NewPartFromText("What is the difference between each of the " +
                                      "main benchmarks between these two papers? " +
                                      "Output these in a table."),
            }
            contents := []*genai.Content{
                genai.NewContentFromParts(promptParts, genai.RoleUser),
            }
    
            modelName := "gemini-2.0-flash"
            result, _ := client.Models.GenerateContent(
                ctx,
                modelName,
                contents,
                nil,
            )
    
            fmt.Println(result.Text())
        }

## Technical details

Gemini supports a maximum of 1,000 document pages. Document pages must be in one of the following text data MIME types:

-   PDF - `application/pdf`
-   JavaScript - `application/x-javascript`, `text/javascript`
-   Python - `application/x-python`, `text/x-python`
-   TXT - `text/plain`
-   HTML - `text/html`
-   CSS - `text/css`
-   Markdown - `text/md`
-   CSV - `text/csv`
-   XML - `text/xml`
-   RTF - `text/rtf`

Each document page is equivalent to 258 tokens.

While there are no specific limits to the number of pixels in a document besides the model's context window, larger pages are scaled down to a maximum resolution of 3072x3072 while preserving their original aspect ratio, while smaller pages are scaled up to 768x768 pixels. There is no cost reduction for pages at lower sizes, other than bandwidth, or performance improvement for pages at higher resolution.

For best results:

-   Rotate pages to the correct orientation before uploading.
-   Avoid blurry pages.
-   If using a single page, place the text prompt after the page.