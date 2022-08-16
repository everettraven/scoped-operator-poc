package main

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func main() {
	cli, err := client.New(config.GetConfigOrDie(), client.Options{})
	if err != nil {
		fmt.Println("failed to get client")
		os.Exit(1)
	}

	// modify this loop to adjust the namespaces created
	for i := 0; i < 10; i++ {
		nsString := fmt.Sprintf("namespace-%d", i)
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: nsString,
			},
		}

		err := cli.Create(context.TODO(), ns, &client.CreateOptions{})
		if err != nil {
			fmt.Println("encountered an error creating namespace", ns.Name, "| ERROR:", err.Error())
			continue
		}

		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("rb-ns-%d", i),
				Namespace: nsString,
			},

			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "scoped-memcached-operator-manager-role",
			},

			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "scoped-memcached-operator-controller-manager",
					Namespace: "scoped-memcached-operator-system",
				},
			},
		}

		err = cli.Create(context.TODO(), rb, &client.CreateOptions{})
		if err != nil {
			fmt.Println("encountered an error creating rolebinding", rb.Name, "| ERROR:", err.Error())
			continue
		}
	}
}
