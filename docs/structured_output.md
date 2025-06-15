# Structured output

-   On this page
-   [Generating JSON](https://ai.google.dev/gemini-api/docs/structured-output#generating-json)
    -   [Configuring a schema (recommended)](https://ai.google.dev/gemini-api/docs/structured-output#configuring-a-schema)
    -   [Providing a schema in a text prompt](https://ai.google.dev/gemini-api/docs/structured-output#schema-in-text-prompt)
-   [Generating enum values](https://ai.google.dev/gemini-api/docs/structured-output#generating-enums)
-   [About JSON schemas](https://ai.google.dev/gemini-api/docs/structured-output#json-schemas)
    -   [Property ordering](https://ai.google.dev/gemini-api/docs/structured-output#property-ordering)
    -   [Schemas in Python](https://ai.google.dev/gemini-api/docs/structured-output#schemas-in-python)
    -   [JSON Schema support](https://ai.google.dev/gemini-api/docs/structured-output#json-schema)
-   [Best practices](https://ai.google.dev/gemini-api/docs/structured-output#considerations)
-   [What's next](https://ai.google.dev/gemini-api/docs/structured-output#whats-next)

You can configure Gemini for structured output instead of unstructured text, allowing precise extraction and standardization of information for further processing. For example, you can use structured output to extract information from resumes, standardize them to build a structured database.

Gemini can generate either [JSON](https://ai.google.dev/gemini-api/docs/structured-output#generating-json) or [enum values](https://ai.google.dev/gemini-api/docs/structured-output#generating-enums) as structured output.

## Generating JSON

There are two ways to generate JSON using the Gemini API:

-   Configure a schema on the model
-   Provide a schema in a text prompt

Configuring a schema on the model is the **recommended** way to generate JSON, because it constrains the model to output JSON.

### Configuring a schema (recommended)

To constrain the model to generate JSON, configure a `responseSchema`. The model will then respond to any prompt with JSON-formatted output.

```go
package main
    
    import (
        "context"
        "fmt"
        "log"
    
        "google.golang.org/genai"
    )
    
    func main() {
        ctx := context.Background()
        client, err := genai.NewClient(ctx, &genai.ClientConfig{
            APIKey:  "GOOGLE_API_KEY",
            Backend: genai.BackendGeminiAPI,
        })
        if err != nil {
            log.Fatal(err)
        }
    
        config := &genai.GenerateContentConfig{
            ResponseMIMEType: "application/json",
            ResponseSchema: &genai.Schema{
                Type: genai.TypeArray,
                Items: &genai.Schema{
                    Type: genai.TypeObject,
                    Properties: map[string]*genai.Schema{
                        "recipeName": {Type: genai.TypeString},
                        "ingredients": {
                            Type:  genai.TypeArray,
                            Items: &genai.Schema{Type: genai.TypeString},
                        },
                    },
                    PropertyOrdering: []string{"recipeName", "ingredients"},
                },
            },
        }
    
        result, err := client.Models.GenerateContent(
            ctx,
            "gemini-2.0-flash",
            genai.Text("List a few popular cookie recipes, and include the amounts of ingredients."),
            config,
        )
        if err != nil {
            log.Fatal(err)
        }
        fmt.Println(result.Text())
    }
```

The output might look like this:

`[ { "recipeName": "Chocolate Chip Cookies", "ingredients": [ "1 cup (2 sticks) unsalted butter, softened", "3/4 cup granulated sugar", "3/4 cup packed brown sugar", "1 teaspoon vanilla extract", "2 large eggs", "2 1/4 cups all-purpose flour", "1 teaspoon baking soda", "1 teaspoon salt", "2 cups chocolate chips" ] }, ... ]`