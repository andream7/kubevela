"envbinding": {
	annotations: {}
	description: "Determining the destination where components should be deployed to, and support override configuration"
	labels: {
		"deprecated": "true"
	}
	attributes: {}
	type: "policy"
}

template: {

	#PatchParams: {
		// +usage=Specify the name of the patch component, if empty, all components will be merged
		name?: string
		// +usage=Specify the type of the patch component.
		type?: string
		properties?: {...}
		traits?: [...{
			type: string
			properties?: {...}
			// +usage=Specify if the trait shoued be remove, default false
			disable: *false | bool
		}]
	}

	parameter: {
		envs: [...{
			name: string
			placement?: {
				clusterSelector?: {
					// +usage=Specify cluster name, defualt local
					name: *"local" | string
					labels?: [string]: string
				}
				namespaceSelector?: {
					// +usage=Specify namespace name.
					name?: string
					labels?: [string]: string
				}
			}
			selector?: {
				components: [...string]
			}
			patch?: {
				components: [...#PatchParams]
			}
		}]
	}
}
