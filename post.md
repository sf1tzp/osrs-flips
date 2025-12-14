Hello /r/ollama,

It's pretty amazing how many projects the community has shared here, I wanted to add my own. I've had a lot of fun learning about LLMs this summer in part due to all of you. Thanks!

This is a relatively straightforward program, that gets trading data for the online game OldSchool RuneScape's marketplace, applies some configurable filters for price and volume criteria, then has ollama generate a nicely formatted markdown document presenting the data. You can generate these on the fly with the CLI, or deploy a container to run jobs.

On the prompt engineering side, I've iterated a bit on the task and the formatting. The program leverages few-shot learning's which can be configured in relevant markdown files. Previously, the prompt was a bit too verbose and confusing, which led to inconsistent results. I'm pretty happy with the output formatting now, and it's nice to have those wiki links "for free" - without needing to write additional templates or wasted as context.

I also iterated on the "data presentation" problem a bit. I tried plaintext table formats, but found the models would often have trouble associating cell values with column names in large datasets. Next, a simple column name -> value json mapping, to keep the concept close to the value. Unfortunately that's extremely inefficient, context wise... so I settled on a nested json format, attempting to keep similar concepts and values together under nested keys. This seems to work well but my implementation is not perfect.

Anyways, thanks for reading! If you'd like to take a closer look, the repository is https://github.com/sf1tzp/osrs-flips



