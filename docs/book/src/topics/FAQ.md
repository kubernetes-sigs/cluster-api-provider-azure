# Frequently Asked Questions

## Does CAPZ support Feature X?
The best way to check if CAPZ supports a feature is by reviewing [the public milestones](https://github.com/kubernetes-sigs/cluster-api-provider-azure/milestones). The public milestones will also provide insight into what's coming in the 
next 1-2 months. All open items for the next milestone are displayed in the [Milestone-Open project board](https://github.com/orgs/kubernetes-sigs/projects/26/views/7), which is updated at the start of each 2-month release cycle. Planning and
discussions for these milestones typically happening during the [Cluster API Azure Office Hours](https://docs.google.com/document/d/1P2FrRjuCZjGy0Yh72lwWCwmXekSEkqliUVTmJy_ETIk/edit?tab=t.0) (every Thursday at 9am PT) after each major
release.

You can also check the [GitHub repository](https://github.com/kubernetes-sigs/cluster-api-provider-azure) issues and milestones for more details on specific features. 


## How can I enable Feature X if it's not?
If CAPZ does not currently support Feature X, you have a couple of options:
1. **Check for Experimental Features**: Sometimes features not fully supported are available as experimental. You can experiment with said features by enabling them using feature gates. Refer to the 
[Experimental Features](https://cluster-api.sigs.k8s.io/tasks/experimental-features/experimental-features) section in the CAPI documentation for detailed instructions.
2. **Contribute to CAPZ**: If you're eager to use Feature X, consider contributing to the CAPZ project. Our community welcomes contributions, and your input can help accelerate support for new features.

## Why doesn't CAPZ support Feature X?
CAPZ prioritizes features based on community demand, relevance to Azure Kubernetes deployments, and resource availability. Thus, Feature X may not be supported because:
- **Limited Demand:** There might be insufficient demand from the community to prioritize its development.
- **Technical Constraints:** Integrating Feature X could present technical challenges that require more time and resources.
- **Roadmap Alignment:** Feature X may not align with our current strategic roadmap or immediate goals.
We continuously evaluate and update our roadmap based on user feedback and evolving requirements, so your input is valuable in shaping future support. 

## How do I add Feature X to CAPZ?
To add Feature X to CAPZ, consider following these steps:
1. **Review the Roadmap:** Ensure that Feature X aligns with the CAPZ roadmap and that it's not already planned or in development.
2. **Submit an Issue:** Open an issue on our [GitHub Repository](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues) to discuss Feature X with the maintainers and the community. P;ease provid detailed information about the
feature and its benefits.
3. **Contribute Code:** If you have the capabilities, you can implement Feature X yourself. Fork the repository, develop the feature following our contribution guidelines, and submit a pull request for review. 
4. **Collaborate with the Community:** Engage with and receive updates from other contributors and maintainers through our [Slack channel](https://kubernetes.slack.com/messages/CEX9HENG7) or 
[mailing lists](https://groups.google.com/forum/#!forum/kubernetes-sig-cluster-lifecycle) to gather support and feedback for Feature X.
By actively participating, you can enhance CAPZ's functionalilty and ensure it meets the needs of the Kubernetes community on Azure. 
